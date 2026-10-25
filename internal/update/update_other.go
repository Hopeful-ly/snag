//go:build !linux && !darwin && !windows

package update

import "errors"

func kind() string { return "" }

func install(string) error { return errors.New("this copy of Snag can't update itself") }
