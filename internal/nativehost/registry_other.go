//go:build !windows

package nativehost

import "errors"

var errNoRegistry = errors.New("only Windows has a registry")

func setRegistry(string, string) error { return errNoRegistry }
func deleteRegistry(string) error      { return errNoRegistry }
