// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package main

import (
	"encoding/json"
	"errors"
	"strconv"
)

type playgroundJSONScanner struct {
	data string
	at   int
}

func playgroundJSONValueSpan(data string, parts []string) (int, int, error) {
	scanner := playgroundJSONScanner{data: data}
	return scanner.locate(parts)
}

func (scanner *playgroundJSONScanner) locate(parts []string) (int, int, error) {
	scanner.space()
	start := scanner.at
	if len(parts) == 0 {
		if err := scanner.skipValue(); err != nil {
			return 0, 0, err
		}
		return start, scanner.at, nil
	}
	if scanner.at >= len(scanner.data) {
		return 0, 0, errors.New("missing value")
	}
	switch scanner.data[scanner.at] {
	case '{':
		return scanner.locateObject(parts)
	case '[':
		return scanner.locateArray(parts)
	default:
		return 0, 0, errors.New("scalar has no child")
	}
}

func (scanner *playgroundJSONScanner) locateObject(parts []string) (int, int, error) {
	scanner.at++
	for {
		scanner.space()
		if scanner.take('}') {
			return 0, 0, errors.New("missing object member")
		}
		keyStart := scanner.at
		if err := scanner.skipString(); err != nil {
			return 0, 0, err
		}
		var key string
		if err := json.Unmarshal([]byte(scanner.data[keyStart:scanner.at]), &key); err != nil {
			return 0, 0, err
		}
		scanner.space()
		if !scanner.take(':') {
			return 0, 0, errors.New("missing object colon")
		}
		if key == parts[0] {
			return scanner.locate(parts[1:])
		}
		if err := scanner.skipValue(); err != nil {
			return 0, 0, err
		}
		scanner.space()
		if scanner.take('}') {
			return 0, 0, errors.New("missing object member")
		}
		if !scanner.take(',') {
			return 0, 0, errors.New("missing object comma")
		}
	}
}

func (scanner *playgroundJSONScanner) locateArray(parts []string) (int, int, error) {
	wanted, err := strconv.Atoi(parts[0])
	if err != nil || wanted < 0 || (len(parts[0]) > 1 && parts[0][0] == '0') {
		return 0, 0, errors.New("invalid array index")
	}
	scanner.at++
	for index := 0; ; index++ {
		scanner.space()
		if scanner.take(']') {
			return 0, 0, errors.New("array index is outside the value")
		}
		if index == wanted {
			return scanner.locate(parts[1:])
		}
		if err := scanner.skipValue(); err != nil {
			return 0, 0, err
		}
		scanner.space()
		if scanner.take(']') {
			return 0, 0, errors.New("array index is outside the value")
		}
		if !scanner.take(',') {
			return 0, 0, errors.New("missing array comma")
		}
	}
}

func (scanner *playgroundJSONScanner) skipValue() error {
	scanner.space()
	if scanner.at >= len(scanner.data) {
		return errors.New("missing value")
	}
	switch scanner.data[scanner.at] {
	case '"':
		return scanner.skipString()
	case '{':
		scanner.at++
		scanner.space()
		if scanner.take('}') {
			return nil
		}
		for {
			if err := scanner.skipString(); err != nil {
				return err
			}
			scanner.space()
			if !scanner.take(':') {
				return errors.New("missing object colon")
			}
			if err := scanner.skipValue(); err != nil {
				return err
			}
			scanner.space()
			if scanner.take('}') {
				return nil
			}
			if !scanner.take(',') {
				return errors.New("missing object comma")
			}
			scanner.space()
		}
	case '[':
		scanner.at++
		scanner.space()
		if scanner.take(']') {
			return nil
		}
		for {
			if err := scanner.skipValue(); err != nil {
				return err
			}
			scanner.space()
			if scanner.take(']') {
				return nil
			}
			if !scanner.take(',') {
				return errors.New("missing array comma")
			}
		}
	default:
		start := scanner.at
		for scanner.at < len(scanner.data) {
			switch scanner.data[scanner.at] {
			case ' ', '\t', '\r', '\n', ',', ']', '}':
				if scanner.at == start {
					return errors.New("missing scalar")
				}
				return nil
			default:
				scanner.at++
			}
		}
		if scanner.at == start {
			return errors.New("missing scalar")
		}
		return nil
	}
}

func (scanner *playgroundJSONScanner) skipString() error {
	if !scanner.take('"') {
		return errors.New("missing string")
	}
	for scanner.at < len(scanner.data) {
		switch scanner.data[scanner.at] {
		case '\\':
			scanner.at += 2
		case '"':
			scanner.at++
			return nil
		default:
			scanner.at++
		}
	}
	return errors.New("unterminated string")
}

func (scanner *playgroundJSONScanner) space() {
	for scanner.at < len(scanner.data) {
		switch scanner.data[scanner.at] {
		case ' ', '\t', '\r', '\n':
			scanner.at++
		default:
			return
		}
	}
}

func (scanner *playgroundJSONScanner) take(want byte) bool {
	if scanner.at >= len(scanner.data) || scanner.data[scanner.at] != want {
		return false
	}
	scanner.at++
	return true
}
