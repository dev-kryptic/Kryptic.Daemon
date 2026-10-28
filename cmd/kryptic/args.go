package main

import (
	"fmt"
	"os"
	"strings"
)

func hasArg(name string) bool {
	for _, arg := range os.Args[2:] {
		if arg == name {
			return true
		}
	}
	return false
}

func argValue(name string) (string, bool) {
	args := os.Args[2:]
	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			return args[i+1], true
		}
		if strings.HasPrefix(arg, name+"=") {
			return strings.TrimPrefix(arg, name+"="), true
		}
	}
	return "", false
}

func confirmed(question string, details ...string) bool {
	for _, arg := range os.Args[2:] {
		if arg == "--yes" || arg == "-y" {
			return true
		}
	}
	stat, err := os.Stdin.Stat()
	if err != nil || stat.Mode()&os.ModeCharDevice == 0 {
		return true
	}
	for _, line := range details {
		fmt.Println(line)
	}
	fmt.Print(question)
	var answer string
	_, _ = fmt.Scanln(&answer)
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}
