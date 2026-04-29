package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const usage = `Sprint Tracker.

Usage:
    go run docs/sprints/tracker.go stats                    # Show overview
    go run docs/sprints/tracker.go current                  # Show in_progress sprint
    go run docs/sprints/tracker.go next                     # Show next planned sprint
    go run docs/sprints/tracker.go add 003 "Sprint Title"   # Add new sprint
    go run docs/sprints/tracker.go start 001                # Mark as in_progress
    go run docs/sprints/tracker.go complete 001             # Mark as completed
    go run docs/sprints/tracker.go skip 001                 # Mark as skipped
    go run docs/sprints/tracker.go status 001 completed     # Set arbitrary status
    go run docs/sprints/tracker.go list [--status planned]  # List sprints
    go run docs/sprints/tracker.go sync                     # Sync from .md files
`

var validStatuses = map[string]bool{
	"planned":     true,
	"in_progress": true,
	"completed":   true,
	"skipped":     true,
}

var statusOrder = []string{"planned", "in_progress", "completed", "skipped"}

var statusIcons = map[string]string{
	"planned":     " ",
	"in_progress": "*",
	"completed":   "+",
	"skipped":     "-",
}

type SprintEntry struct {
	SprintID  string
	Title     string
	Status    string
	CreatedAt string
	UpdatedAt string
}

func normalizeID(id string) string {
	n, err := strconv.Atoi(id)
	if err != nil {
		return id
	}
	return fmt.Sprintf("%03d", n)
}

func (e *SprintEntry) SprintNumber() int {
	n, _ := strconv.Atoi(e.SprintID)
	return n
}

func (e *SprintEntry) DocPath() string {
	return fmt.Sprintf("docs/sprints/SPRINT-%s.md", e.SprintID)
}

func (e *SprintEntry) ToTSV() string {
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s", e.SprintID, e.Title, e.Status, e.CreatedAt, e.UpdatedAt)
}

func parseTSVLine(line string) (*SprintEntry, error) {
	parts := strings.Split(line, "\t")
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid TSV line: %s", line)
	}
	status := parts[2]
	if !validStatuses[status] {
		return nil, fmt.Errorf("invalid status: %s", status)
	}
	return &SprintEntry{
		SprintID:  normalizeID(parts[0]),
		Title:     parts[1],
		Status:    status,
		CreatedAt: parts[3],
		UpdatedAt: parts[4],
	}, nil
}

const tsvHeader = "sprint_id\ttitle\tstatus\tcreated_at\tupdated_at"

type SprintTracker struct {
	path    string
	entries map[string]*SprintEntry
}

func newTracker(path string) *SprintTracker {
	return &SprintTracker{path: path, entries: make(map[string]*SprintEntry)}
}

func (t *SprintTracker) load() error {
	f, err := os.Open(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if first {
			first = false
			continue // skip header
		}
		if line == "" {
			continue
		}
		entry, err := parseTSVLine(line)
		if err != nil {
			return err
		}
		t.entries[entry.SprintID] = entry
	}
	return scanner.Err()
}

func (t *SprintTracker) sorted() []*SprintEntry {
	entries := make([]*SprintEntry, 0, len(t.entries))
	for _, e := range t.entries {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].SprintNumber() < entries[j].SprintNumber()
	})
	return entries
}

func (t *SprintTracker) save() error {
	f, err := os.Create(t.path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintln(f, tsvHeader)
	for _, e := range t.sorted() {
		fmt.Fprintln(f, e.ToTSV())
	}
	return nil
}

func now() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func (t *SprintTracker) add(sprintID, title, status string) (*SprintEntry, error) {
	sprintID = normalizeID(sprintID)
	if !validStatuses[status] {
		return nil, fmt.Errorf("invalid status: %s. Must be one of %v", status, statusOrder)
	}
	if _, exists := t.entries[sprintID]; exists {
		return nil, fmt.Errorf("Sprint %s already exists", sprintID)
	}
	ts := now()
	entry := &SprintEntry{
		SprintID:  sprintID,
		Title:     title,
		Status:    status,
		CreatedAt: ts,
		UpdatedAt: ts,
	}
	t.entries[sprintID] = entry
	return entry, nil
}

func (t *SprintTracker) updateStatus(sprintID, status string) (*SprintEntry, error) {
	sprintID = normalizeID(sprintID)
	if !validStatuses[status] {
		return nil, fmt.Errorf("invalid status: %s. Must be one of %v", status, statusOrder)
	}
	entry, exists := t.entries[sprintID]
	if !exists {
		return nil, fmt.Errorf("Sprint %s not found", sprintID)
	}
	entry.Status = status
	entry.UpdatedAt = now()
	return entry, nil
}

func (t *SprintTracker) getInProgress() *SprintEntry {
	for _, e := range t.entries {
		if e.Status == "in_progress" {
			return e
		}
	}
	return nil
}

func (t *SprintTracker) getNextPlanned() *SprintEntry {
	var best *SprintEntry
	for _, e := range t.entries {
		if e.Status == "planned" {
			if best == nil || e.SprintNumber() < best.SprintNumber() {
				best = e
			}
		}
	}
	return best
}

func (t *SprintTracker) getByStatus(status string) []*SprintEntry {
	var result []*SprintEntry
	for _, e := range t.sorted() {
		if e.Status == status {
			result = append(result, e)
		}
	}
	return result
}

func (t *SprintTracker) countByStatus() map[string]int {
	counts := make(map[string]int)
	for _, s := range statusOrder {
		counts[s] = 0
	}
	for _, e := range t.entries {
		counts[e.Status]++
	}
	return counts
}

var titlePattern = regexp.MustCompile(`(?m)^# Sprint (\d+): (.+)$`)

func (t *SprintTracker) syncFromDocs() ([]string, error) {
	dir := filepath.Dir(t.path)
	matches, err := filepath.Glob(filepath.Join(dir, "SPRINT-*.md"))
	if err != nil {
		return nil, err
	}

	fnPattern := regexp.MustCompile(`SPRINT-(\d+)\.md$`)
	var changes []string

	for _, mdFile := range matches {
		fnMatch := fnPattern.FindStringSubmatch(filepath.Base(mdFile))
		if fnMatch == nil {
			continue
		}
		sprintID := normalizeID(fnMatch[1])

		content, err := os.ReadFile(mdFile)
		if err != nil {
			continue
		}

		title := fmt.Sprintf("Sprint %s", sprintID)
		if m := titlePattern.FindSubmatch(content); m != nil {
			title = strings.TrimSpace(string(m[2]))
		}

		if _, exists := t.entries[sprintID]; !exists {
			if _, err := t.add(sprintID, title, "planned"); err == nil {
				changes = append(changes, fmt.Sprintf("Added: %s - %s", sprintID, title))
			}
		} else {
			existing := t.entries[sprintID]
			if existing.Title != title {
				existing.Title = title
				existing.UpdatedAt = now()
				changes = append(changes, fmt.Sprintf("Updated title: %s - %s", sprintID, title))
			}
		}
	}
	return changes, nil
}

func printEntry(e *SprintEntry, verbose bool) {
	icon := statusIcons[e.Status]
	if icon == "" {
		icon = "?"
	}
	fmt.Printf("[%s] %s: %s\n", icon, e.SprintID, e.Title)
	if verbose {
		fmt.Printf("    Status: %s\n", e.Status)
		fmt.Printf("    Doc: %s\n", e.DocPath())
		fmt.Printf("    Created: %s\n", e.CreatedAt)
		fmt.Printf("    Updated: %s\n", e.UpdatedAt)
	}
}

func resolveTSVPath() string {
	primary := filepath.Join("docs", "sprints", "tracker.tsv")
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	// One-time migration fallback
	legacy := filepath.Join("docs", "sprints", "ledger.tsv")
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	// Default to primary (will be created on save)
	return primary
}

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(1)
	}

	cmd := os.Args[1]
	tsvPath := resolveTSVPath()
	tracker := newTracker(tsvPath)
	if err := tracker.load(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// If we loaded from legacy path, switch to new path for saving
	tracker.path = filepath.Join("docs", "sprints", "tracker.tsv")

	switch cmd {
	case "stats":
		counts := tracker.countByStatus()
		total := 0
		for _, c := range counts {
			total += c
		}
		fmt.Println("Sprint Ledger Statistics")
		fmt.Println("========================")
		fmt.Printf("Total sprints: %d\n", total)
		fmt.Println()
		for _, s := range statusOrder {
			fmt.Printf("  %s: %d\n", s, counts[s])
		}
		if current := tracker.getInProgress(); current != nil {
			fmt.Println()
			fmt.Printf("Current: %s - %s\n", current.SprintID, current.Title)
		}
		if next := tracker.getNextPlanned(); next != nil {
			fmt.Printf("Next: %s - %s\n", next.SprintID, next.Title)
		}

	case "current":
		if current := tracker.getInProgress(); current != nil {
			printEntry(current, true)
		} else {
			fmt.Println("No sprint currently in progress")
		}

	case "next":
		if next := tracker.getNextPlanned(); next != nil {
			printEntry(next, true)
		} else {
			fmt.Println("No planned sprints")
		}

	case "list":
		var status string
		for i, arg := range os.Args {
			if arg == "--status" && i+1 < len(os.Args) {
				status = os.Args[i+1]
				break
			}
		}
		var entries []*SprintEntry
		if status != "" {
			entries = tracker.getByStatus(status)
		} else {
			entries = tracker.sorted()
		}
		if len(entries) == 0 {
			fmt.Println("No sprints found")
		} else {
			for _, e := range entries {
				printEntry(e, false)
			}
		}

	case "add":
		if len(os.Args) < 4 {
			fmt.Println("Usage: tracker.go add <sprint_id> <title>")
			os.Exit(1)
		}
		title := strings.Join(os.Args[3:], " ")
		entry, err := tracker.add(os.Args[2], title, "planned")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := tracker.save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Added sprint %s: %s\n", entry.SprintID, entry.Title)

	case "start":
		if len(os.Args) < 3 {
			fmt.Println("Usage: tracker.go start <sprint_id>")
			os.Exit(1)
		}
		entry, err := tracker.updateStatus(os.Args[2], "in_progress")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := tracker.save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Started sprint %s: %s\n", entry.SprintID, entry.Title)

	case "complete":
		if len(os.Args) < 3 {
			fmt.Println("Usage: tracker.go complete <sprint_id>")
			os.Exit(1)
		}
		entry, err := tracker.updateStatus(os.Args[2], "completed")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := tracker.save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Completed sprint %s: %s\n", entry.SprintID, entry.Title)

	case "skip":
		if len(os.Args) < 3 {
			fmt.Println("Usage: tracker.go skip <sprint_id>")
			os.Exit(1)
		}
		entry, err := tracker.updateStatus(os.Args[2], "skipped")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := tracker.save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Skipped sprint %s: %s\n", entry.SprintID, entry.Title)

	case "status":
		if len(os.Args) < 4 {
			fmt.Println("Usage: tracker.go status <sprint_id> <status>")
			os.Exit(1)
		}
		entry, err := tracker.updateStatus(os.Args[2], os.Args[3])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := tracker.save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Updated sprint %s to %s\n", entry.SprintID, entry.Status)

	case "sync":
		changes, err := tracker.syncFromDocs()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(changes) > 0 {
			if err := tracker.save(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Sync complete:")
			for _, c := range changes {
				fmt.Printf("  %s\n", c)
			}
		} else {
			fmt.Println("No changes needed")
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Print(usage)
		os.Exit(1)
	}
}
