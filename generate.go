package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)


const realCommit = "Add script and image file"
const fakePrefix = "FAKE_COMMIT"
const dailyCommits = 100
const DurationDay = time.Hour * 24
const DurationWeek = DurationDay * 7


type Pattern struct {
	W, H int
	data []bool
}

// Returns index of point, or -1 if out of bounds:
func (p *Pattern) getIdx(x, y int) int {
	if y < 0 || y >= p.H {
		return -1
	}
	for ; x < 0 ; x += p.W {}

	return y * p.W + x % p.W
}

func (p *Pattern) Draw(start time.Time, numWeeks int) error {
	drawOrigin := time.Date(2015, time.April, 26, 12, 0, 0, 0, time.UTC)
	offset := int(start.Sub(drawOrigin).Hours() / 24 / 7 + 0.5)
	
	d := start.Add(-DurationDay)
	for x:=offset; x<offset+numWeeks; x++ {
		for y:=0; y<7; y++ {
			d = d.Add(DurationDay)
			dataIdx := p.getIdx(x, y)
			if dataIdx < 0 || !p.data[dataIdx] {
				log.Println("Skipping commits for", d)
				continue
			}
			log.Println("Building commits for", d)
			for n:=1; n<=dailyCommits; n++ {
				err := forgeCommit(d, n)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}


func newPatternFromFile(fpath string) (*Pattern, error) {
	contents, err := ioutil.ReadFile(fpath)
	if err != nil {
		return nil, err
	}

	parsed := strings.Split(string(contents), "\n")
	if len(parsed) > 7 {
		return nil, fmt.Errorf("File %s has too many lines: %d", fpath, len(parsed))
	}

	maxWidth := -1
	for _, line := range parsed {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	for idx, line := range parsed {
		parsed[idx] = Pad(line, maxWidth)
	}

	out := new(Pattern)
	out.H = len(parsed)
	out.W = maxWidth
	out.data = make([]bool, out.W * out.H)

	for x:=0; x<out.W; x++ {
		for y:=0; y<out.H; y++ {
			hasSomething := parsed[y][x] != ' '
			out.data[out.getIdx(x, y)] = hasSomething
		}
	}

	return out, nil
}

func Pad(str string, size int) string {
	for ; len(str) < size ; {
		str += " "
	}
	return str
}

func forgeCommit(date time.Time, num int) error {
	year, month, day := date.Date()
	
	msg := fmt.Sprintf("%s: %4d-%02d-%02d #%d", fakePrefix, year, month, day, num)
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", msg)

	// GIT_AUTHOR_DATE and GIT_COMMITTER_DATE
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_AUTHOR_DATE=%s", date.Format(time.RFC3339)))
	cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_COMMITTER_DATE=%s", date.Format(time.RFC3339)))

	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		return err
	}
	return nil
}

// Will squash the entire repo into a single commit: 
func squashHistory() error {
	var err error

	// Drop all commits, but leave directory contents:
	_, err = exec.Command("git", "update-ref", "-d", "HEAD").Output()
	if err != nil {
		return err
	}

	// Recommit directory:
	_, err = exec.Command("git", "commit", "-am", realCommit).Output()
	if err != nil {
		return err
	}

	// Trigger a gc, since we just orphaned a LOT of objects:
	_, err = exec.Command("git", "gc", "--aggressive", "--force", "--prune=now").Output()
	if err != nil {
		return err
	}

	return nil
}

func getOrigin(weeksAgo int) time.Time {
	year, month, day := time.Now().UTC().Date()
	out := time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
	out = out.Add(-time.Duration(out.Weekday()) * DurationDay)
	return out.Add(-time.Duration(weeksAgo) * DurationWeek)
}

func main() {
	var err error
	var fileName string
	var numWeeks int
	var resetRepo bool
	var showHelp bool

	flag.StringVar(&fileName, "pattern", "", "The pattern file to use. (REQUIRED)")
	flag.IntVar(&numWeeks, "weeks", 1, "The number of weeks to generate. Default: 1")
	flag.BoolVar(&resetRepo, "reset", false, "Whether to reset the git repo first. Default: false")
	flag.BoolVar(&showHelp, "help", false, "Shows this help message")
	flag.Parse()

	if showHelp {
		flag.PrintDefaults()
		return
	}

	if fileName == "" {
		log.Fatal("Pattern file is required")
	}

	log.Println("Loading pattern file...")
	pattern, err := newPatternFromFile(fileName)
	if err != nil {
		log.Fatal("Couldn't load pattern:", err)
	}

	if resetRepo {
		log.Println("Squashing prior history into a single commit...")
		err = squashHistory()
		if err != nil {
			log.Fatal("Couldn't flatten history:", err)
		}
	}

	d := getOrigin(numWeeks)
	
	err = pattern.Draw(d, numWeeks)
	if err != nil {
		log.Fatal("Couldn't draw pattern:", err)
	}
}