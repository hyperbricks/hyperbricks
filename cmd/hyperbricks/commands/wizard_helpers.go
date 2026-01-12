package commands

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func promptYesNo(reader *bufio.Reader, prompt string) (bool, error) {
	return promptYesNoDefault(reader, prompt, false)
}

func promptYesNoDefault(reader *bufio.Reader, prompt string, defaultYes bool) (bool, error) {
	for {
		fmt.Print(prompt)
		input, err := readLine(reader)
		if err != nil {
			return false, err
		}
		input = strings.ToLower(strings.TrimSpace(input))
		if input == "" {
			return defaultYes, nil
		}
		if input == "y" || input == "yes" {
			return true, nil
		}
		if input == "n" || input == "no" {
			return false, nil
		}
		fmt.Println("Please enter y or n.")
	}
}

func promptInput(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	return readLine(reader)
}

func promptPort(reader *bufio.Reader, prompt string, defaultPort int) (int, error) {
	for {
		fmt.Print(prompt)
		input, err := readLine(reader)
		if err != nil {
			return 0, err
		}
		input = strings.TrimSpace(input)
		if input == "" {
			return defaultPort, nil
		}
		port, err := strconv.Atoi(input)
		if err != nil || port <= 0 || port > 65535 {
			fmt.Println("Please enter a valid port number.")
			continue
		}
		return port, nil
	}
}

func readLine(reader *bufio.Reader) (string, error) {
	var buf []byte
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF && len(buf) > 0 {
				return string(buf), nil
			}
			return "", err
		}
		if b == '\n' {
			break
		}
		if b == '\r' {
			if next, err := reader.Peek(1); err == nil && len(next) > 0 && next[0] == '\n' {
				_, _ = reader.ReadByte()
			}
			break
		}
		buf = append(buf, b)
	}
	return string(buf), nil
}
