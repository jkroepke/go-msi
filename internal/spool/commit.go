package spool

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// Move renames the finished file From onto To, which may exist.
type Move struct {
	From string
	To   string
}

// moved is a Move that Commit made, with the backup of the file it replaced.
type moved struct {
	Move

	backup string
}

// Commit makes every move or none: each existing destination is first
// renamed to a backup beside it, and when a later move fails, the moves made
// so far are undone and the backups restored. The backups are removed once
// every move succeeded. Each From must be on the file system of its To.
func Commit(moves []Move) error {
	done := make([]moved, 0, len(moves))

	for _, move := range moves {
		step, err := commitOne(move)
		if err != nil {
			return errors.Join(err, rollback(done))
		}

		done = append(done, step)
	}

	for _, step := range done {
		if step.backup != "" {
			_ = os.Remove(step.backup)
		}
	}

	return nil
}

// commitOne backs up To, if it exists, and renames From onto it.
func commitOne(move Move) (moved, error) {
	step := moved{Move: move}

	if _, err := os.Lstat(move.To); err == nil {
		backup, err := os.CreateTemp(filepath.Dir(move.To), ".msi-builder-*.old")
		if err != nil {
			return step, fmt.Errorf("back up %s: %w", move.To, err)
		}

		_ = backup.Close()

		if err := os.Rename(move.To, backup.Name()); err != nil {
			_ = os.Remove(backup.Name())

			return step, fmt.Errorf("back up %s: %w", move.To, err)
		}

		step.backup = backup.Name()
	}

	if err := os.Rename(move.From, move.To); err != nil {
		err = fmt.Errorf("move %s to %s: %w", filepath.Base(move.From), move.To, err)

		if step.backup != "" {
			if restore := os.Rename(step.backup, move.To); restore != nil {
				err = errors.Join(err, fmt.Errorf("restore %s from %s: %w", move.To, step.backup, restore))
			}
		}

		return step, err
	}

	return step, nil
}

// rollback undoes done in reverse order: each new file goes back to its
// From, and the backup, if any, to its To.
func rollback(done []moved) error {
	var errs []error

	for _, step := range slices.Backward(done) {
		if err := os.Rename(step.To, step.From); err != nil {
			errs = append(errs, fmt.Errorf("undo %s: %w", step.To, err))

			continue
		}

		if step.backup != "" {
			if err := os.Rename(step.backup, step.To); err != nil {
				errs = append(errs, fmt.Errorf("restore %s from %s: %w", step.To, step.backup, err))
			}
		}
	}

	return errors.Join(errs...)
}
