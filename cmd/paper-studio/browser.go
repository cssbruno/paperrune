// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package main

import (
	"errors"
	"os/exec"
)

func openStudioBrowser(url, goos string) error {
	name, args, err := studioBrowserCommand(url, goos)
	if err != nil {
		return err
	}
	return exec.Command(name, args...).Start() // #nosec G204 -- the executable and argument shape are fixed per supported operating system.
}

func studioBrowserCommand(url, goos string) (string, []string, error) {
	switch goos {
	case "darwin":
		return "open", []string{url}, nil
	case "linux":
		return "xdg-open", []string{url}, nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}, nil
	default:
		return "", nil, errors.New("automatic browser opening is unavailable on this platform")
	}
}
