package rules

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Room-Elephant/pluck/internal/log"
)

type Ruleset map[string]string

// Load reads and parses a rules file. The format is one rule per line:
//
//	label:destination/path
//
// Lines beginning with '#' and blank lines are ignored. Labels are
// normalised to lower-case at load time so matching is case-insensitive.
func Load(path string) (Ruleset, error) {
	rulesFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening rules file: %w", err)
	}
	defer rulesFile.Close()

	activeRuleset := make(Ruleset)

	scanner := bufio.NewScanner(rulesFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		colonIndex := strings.IndexByte(line, ':')
		if colonIndex < 0 {
			log.Errorf("invalid rule (skipping, missing ':'): %s", line)
			continue
		}

		label := strings.ToLower(strings.TrimSpace(line[:colonIndex]))
		directory := strings.TrimSpace(line[colonIndex+1:])

		if label == "" || directory == "" {
			log.Errorf("invalid rule (skipping, empty label or directory): %s", line)
			continue
		}

		activeRuleset[label] = directory
		log.Debugf("rule loaded: %q -> %q", label, directory)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading rules file: %w", err)
	}

	log.Infof("loaded %d rule(s) from %s", len(activeRuleset), path)
	return activeRuleset, nil
}

func (activeRuleset Ruleset) Destination(label string) string {
	return activeRuleset[strings.ToLower(label)]
}
