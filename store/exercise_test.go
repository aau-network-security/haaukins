package store_test

import (
	"errors"
	"testing"

	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/store"
)

type exer struct {
	name string
	tags []string
}

func TestNewExerciseStore(t *testing.T) {
	tt := []struct {
		name string
		in   []exer
		err  string
	}{
		{name: "Normal", in: []exer{{name: "Test", tags: []string{"tst"}}}},
		{name: "Multiple tags", in: []exer{{name: "Test", tags: []string{"tst", "tst2"}}}},
		{name: "Identical tags", in: []exer{
			{name: "Test", tags: []string{"tst"}},
			{name: "Test 2", tags: []string{"tst"}},
		}, err: "Exercise tag already exists: tst"}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var exers []exercise.Config
			var tags []string
			for _, e := range tc.in {
				exers = append(exers, exercise.Config{
					Name: e.name,
					Tags: e.tags,
				})

				tags = append(tags, e.tags...)
			}

			es, err := store.NewExerciseStore(exers)
			if err != nil {
				if tc.err != "" {
					if tc.err != err.Error() {
						t.Fatalf("unexpected error (expected: \"%s\") when creating store: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("received error when creating exercise store, but expected none: %s", err)
			}

			if tc.err != "" {
				t.Fatalf("received no error when expecting: %s", tc.err)
			}

			if n := len(es.ListExercises()); n != len(exers) {
				t.Fatalf("unexpected amount of exercises, expected: %d, got: %d", len(exers), n)
			}

			exercises, err := es.GetExercisesByTags(tags[0], tags[1:]...)
			if err != nil {
				t.Fatalf("unexpected error when looking up tags")
			}

			if n := len(exercises); n != len(tags) {
				t.Fatalf("unexpected amount of exercises when looking up, expected: %d, got: %d", len(tags), n)
			}

		})
	}
}

func TestCreateExercise(t *testing.T) {
	tt := []struct {
		name string
		in   exer
		err  bool
	}{
		{name: "Normal", in: exer{name: "Test", tags: []string{"tst"}}},
		{name: "Invalid tag exercise", in: exer{name: "Test", tags: []string{"tst tst"}}, err: true},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var ran bool
			var count int
			var errToThrow error

			hook := func(e []exercise.Config) error {
				count = len(e)
				ran = true

				return errToThrow
			}

			es, err := store.NewExerciseStore([]exercise.Config{}, hook)
			if err != nil {
				t.Fatalf("received error when creating exercise store, but expected none: %s", err)
			}

			err = es.CreateExercise(exercise.Config{
				Name: tc.in.name,
				Tags: tc.in.tags,
			})
			if err != nil {
				if !tc.err {
					t.Fatalf("unexpected error: %s", err)
				}

				return
			}

			if tc.err {
				t.Fatalf("received no error, but expected error")
			}

			if !ran {
				t.Fatalf("expected hook to have been run")
			}

			if count != 1 {
				t.Fatalf("expected hook to have been run with one exercise")
			}

			es, err = store.NewExerciseStore([]exercise.Config{}, hook)
			if err != nil {
				t.Fatalf("received error when creating exercise store, but expected none: %s", err)
			}

			errToThrow = errors.New("Some error")
			err = es.CreateExercise(exercise.Config{
				Name: tc.in.name,
				Tags: tc.in.tags,
			})
			if err == nil {
				t.Fatalf("expected hook to have thrown error")
			}
		})
	}
}

func TestGetExercises(t *testing.T) {
	tt := []struct {
		name    string
		in      []exer
		lookups []string
		err     string
	}{
		{name: "Normal", in: []exer{
			exer{name: "Test", tags: []string{"tst"}},
		}, lookups: []string{"tst"}},
		{name: "Normal (pool of two)", in: []exer{
			exer{name: "Test", tags: []string{"tst"}},
			exer{name: "Test2", tags: []string{"tst2"}},
		}, lookups: []string{"tst"}},
		{name: "Unknown exercise", in: []exer{},
			err:     "Unknown exercise tag: tst",
			lookups: []string{"tst"}},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var exer []exercise.Config
			for _, e := range tc.in {
				exer = append(exer, exercise.Config{
					Name: e.name,
					Tags: e.tags,
				})
			}

			es, err := store.NewExerciseStore(exer)
			if err != nil {
				t.Fatalf("received error when creating exercise store, but expected none: %s", err)
			}

			exercises, err := es.GetExercisesByTags(tc.lookups[0], tc.lookups[1:]...)
			if err != nil {
				if tc.err != "" {
					if tc.err != err.Error() {
						t.Fatalf("unexpected error (expected: \"%s\") when creating store: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("unexpected error: %s", err)
			}

			if tc.err != "" {
				t.Fatalf("received no error, but expected error")
			}

			if n := len(exercises); n != len(tc.lookups) {
				t.Fatalf("received unexpected amount of exercises (expected: %d): %d", len(tc.lookups), n)
			}
		})
	}
}

func TestDeleteExercise(t *testing.T) {
	tt := []struct {
		name      string
		in        exer
		deleteTag string
		err       string
	}{
		{name: "Normal", in: exer{name: "Test", tags: []string{"tst"}}, deleteTag: "tst"},
		{name: "Unknown exercise", in: exer{name: "Test", tags: []string{"tst"}}, deleteTag: "not-test", err: "Unknown exercise tag: not-test"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			es, err := store.NewExerciseStore([]exercise.Config{
				exercise.Config{
					Name: tc.in.name,
					Tags: tc.in.tags,
				}})
			if err != nil {
				t.Fatalf("received error when creating exercise store, but expected none: %s", err)
			}

			err = es.DeleteExerciseByTag(tc.deleteTag)
			if err != nil {
				if tc.err != "" {
					if tc.err != err.Error() {
						t.Fatalf("unexpected error (expected: \"%s\") when creating store: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("unexpected error: %s", err)
			}

			if tc.err != "" {
				t.Fatalf("received no error, but expected error")
			}
		})
	}
}
