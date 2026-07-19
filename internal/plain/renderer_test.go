package plain

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"meguru/internal/review"
	"meguru/internal/scheduler"
)

type ratedCall struct {
	cardID int64
	rating scheduler.Rating
}

// fakeService mimics review.Service: NextDueCard keeps returning the same
// "current" card until Rate() is called for it, matching real semantics
// closely enough to exercise Run()'s loop.
type fakeService struct {
	current   *review.Card
	remaining []*review.Card
	nextErr   error
	rateErr   error
	rated     []ratedCall
	lastScope review.DeckScope
}

func (f *fakeService) NextDueCard(ctx context.Context, scope review.DeckScope) (*review.Card, error) {
	f.lastScope = scope
	if f.nextErr != nil {
		return nil, f.nextErr
	}
	if f.current == nil {
		if len(f.remaining) == 0 {
			return nil, nil
		}
		f.current = f.remaining[0]
		f.remaining = f.remaining[1:]
	}
	return f.current, nil
}

func (f *fakeService) Rate(ctx context.Context, cardID int64, rating scheduler.Rating, now time.Time) error {
	f.rated = append(f.rated, ratedCall{cardID, rating})
	if f.rateErr != nil {
		return f.rateErr
	}
	f.current = nil
	return nil
}

func TestRun_NothingDuePrintsMessageAndReturnsNil(t *testing.T) {
	svc := &fakeService{}
	var out bytes.Buffer

	err := Run(context.Background(), svc, strings.NewReader(""), &out, review.DeckScope{})

	require.NoError(t, err)
	require.Contains(t, out.String(), "Nothing due right now.")
}

func TestRun_ScopedSession_PassesScopeToNextDueCard(t *testing.T) {
	svc := &fakeService{remaining: []*review.Card{{ID: 1, Expression: "あ", Reading: "a", Meaning: "a"}}}
	var out bytes.Buffer

	err := Run(context.Background(), svc, strings.NewReader("a\ngood\n"), &out,
		review.DeckScope{Slug: "kana-hiragana", Name: "Hiragana"})

	require.NoError(t, err)
	require.Equal(t, review.DeckScope{Slug: "kana-hiragana", Name: "Hiragana"}, svc.lastScope)
}

func TestRun_ScopedSession_NothingDueNamesTheDeck(t *testing.T) {
	svc := &fakeService{}
	var out bytes.Buffer

	err := Run(context.Background(), svc, strings.NewReader(""), &out,
		review.DeckScope{Slug: "jlpt-n5-kanji", Name: "JLPT N5 Kanji"})

	require.NoError(t, err)
	require.Contains(t, out.String(), "Nothing due in JLPT N5 Kanji right now.")
}

func TestRun_NextDueCardErrorPropagates(t *testing.T) {
	svc := &fakeService{nextErr: errors.New("db is on fire")}
	var out bytes.Buffer

	err := Run(context.Background(), svc, strings.NewReader(""), &out, review.DeckScope{})

	require.ErrorContains(t, err, "db is on fire")
}

func TestRun_ShowsCardAndRecordsRating(t *testing.T) {
	svc := &fakeService{remaining: []*review.Card{{ID: 1, Expression: "あ", Reading: "a", Meaning: "a"}}}
	var out bytes.Buffer

	err := Run(context.Background(), svc, strings.NewReader("a\nagain\n"), &out, review.DeckScope{})

	require.NoError(t, err)
	require.Contains(t, out.String(), "Expression:あ")
	require.Contains(t, out.String(), "Reading:   a")
	require.Contains(t, out.String(), "Recorded: Again")
	require.Contains(t, out.String(), "Nothing due right now.")
	require.Equal(t, []ratedCall{{cardID: 1, rating: scheduler.Again}}, svc.rated)
}

// A typed romaji answer that matches the card's reading is reported correct
// before the reveal/rating step (FR-003, Acceptance Scenario 1).
func TestRun_MatchingAnswerReportsCorrect(t *testing.T) {
	svc := &fakeService{remaining: []*review.Card{{ID: 1, Expression: "か", Reading: "ka", Meaning: "ka"}}}
	var out bytes.Buffer

	err := Run(context.Background(), svc, strings.NewReader("ka\ngood\n"), &out, review.DeckScope{})

	require.NoError(t, err)
	require.Contains(t, out.String(), "Correct! (か)")
}

// A typed romaji answer that does not match the card's reading is reported
// as such, but the reveal and rating step still happen (FR-004, FR-005,
// Acceptance Scenario 2).
func TestRun_NonMatchingAnswerRevealsAndStillRates(t *testing.T) {
	svc := &fakeService{remaining: []*review.Card{{ID: 1, Expression: "か", Reading: "ka", Meaning: "ka"}}}
	var out bytes.Buffer

	err := Run(context.Background(), svc, strings.NewReader("shi\ngood\n"), &out, review.DeckScope{})

	require.NoError(t, err)
	require.Contains(t, out.String(), "Not quite — you typed: し")
	require.Contains(t, out.String(), "Reading:   ka")
	require.Equal(t, []ratedCall{{cardID: 1, rating: scheduler.Good}}, svc.rated)
}

func TestRun_AcceptsSingleLetterRating(t *testing.T) {
	svc := &fakeService{remaining: []*review.Card{{ID: 1}}}
	var out bytes.Buffer

	err := Run(context.Background(), svc, strings.NewReader("x\ne\n"), &out, review.DeckScope{})

	require.NoError(t, err)
	require.Equal(t, []ratedCall{{cardID: 1, rating: scheduler.Easy}}, svc.rated)
}

func TestRun_UnrecognizedRatingRetriesSameCard(t *testing.T) {
	svc := &fakeService{remaining: []*review.Card{{ID: 1}}}
	var out bytes.Buffer

	err := Run(context.Background(), svc, strings.NewReader("x\nbogus\nx\ngood\n"), &out, review.DeckScope{})

	require.NoError(t, err)
	require.Contains(t, out.String(), "Unrecognized rating")
	require.Equal(t, []ratedCall{{cardID: 1, rating: scheduler.Good}}, svc.rated)
}

func TestRun_RateErrorPropagates(t *testing.T) {
	svc := &fakeService{remaining: []*review.Card{{ID: 1}}, rateErr: errors.New("write failed")}
	var out bytes.Buffer

	err := Run(context.Background(), svc, strings.NewReader("x\ngood\n"), &out, review.DeckScope{})

	require.ErrorContains(t, err, "write failed")
}

// If stdin closes before an answer is even typed (e.g. the process is
// interrupted right after the card is shown), Run returns cleanly with no
// error — no partial rating is ever submitted (FR-015).
func TestRun_EOFDuringAnswerReadReturnsNilError(t *testing.T) {
	svc := &fakeService{remaining: []*review.Card{{ID: 1}}}
	var out bytes.Buffer

	err := Run(context.Background(), svc, strings.NewReader(""), &out, review.DeckScope{})

	require.NoError(t, err)
	require.Empty(t, svc.rated)
}

// If stdin closes after an answer is typed but before a rating is entered,
// Run also returns cleanly with no error and no partial rating (FR-015).
func TestRun_EOFDuringRatingReadReturnsNilError(t *testing.T) {
	svc := &fakeService{remaining: []*review.Card{{ID: 1}}}
	var out bytes.Buffer

	err := Run(context.Background(), svc, strings.NewReader("x\n"), &out, review.DeckScope{})

	require.NoError(t, err)
	require.Empty(t, svc.rated)
}

func TestRun_EOFDuringRatingRead_UsesRatingShapedAnswerToken(t *testing.T) {
	svc := &fakeService{remaining: []*review.Card{{ID: 1, Reading: "a"}}}
	var out bytes.Buffer

	err := Run(context.Background(), svc, strings.NewReader("good\n"), &out, review.DeckScope{})

	require.NoError(t, err)
	require.Equal(t, []ratedCall{{cardID: 1, rating: scheduler.Good}}, svc.rated)
}

func TestParseRating_AllVariants(t *testing.T) {
	cases := map[string]scheduler.Rating{
		"a": scheduler.Again, "Again": scheduler.Again, " again ": scheduler.Again,
		"h": scheduler.Hard, "hard": scheduler.Hard,
		"g": scheduler.Good, "good": scheduler.Good,
		"e": scheduler.Easy, "easy": scheduler.Easy,
	}
	for input, want := range cases {
		got, ok := parseRating(input)
		require.True(t, ok, "input %q", input)
		require.Equal(t, want, got, "input %q", input)
	}
}

func TestParseRating_Invalid(t *testing.T) {
	_, ok := parseRating("whatever")
	require.False(t, ok)
}

func TestRatingName_AllVariants(t *testing.T) {
	require.Equal(t, "Again", ratingName(scheduler.Again))
	require.Equal(t, "Hard", ratingName(scheduler.Hard))
	require.Equal(t, "Good", ratingName(scheduler.Good))
	require.Equal(t, "Easy", ratingName(scheduler.Easy))
}

func TestRatingName_UnknownRatingReturnsPlaceholder(t *testing.T) {
	require.Equal(t, "?", ratingName(scheduler.Rating(99)))
}
