//go:build !js || !wasm

package main

import (
	"errors"
	"flag"
	"fmt"
	"malaina"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/schollz/progressbar/v3"
)

func main() {
	fid := flag.Int("id", 0, "ID of the anime to start from (Anilist ID)")
	fsearch := flag.String("search", "", "Search for an anime by name (Anilist ID)")
	fimport := flag.String("import", "", "Import a json file")
	fexport := flag.String("export", "", "Export a json file")
	fout := flag.String("o", "medias.html", "Output file")

	flag.Parse()

	var err error
	if *fsearch != "" {
		*fid, err = searchAnimeId(*fsearch)
		if err != nil {
			fmt.Println("Error searching anime:", err)
			os.Exit(1)
		}
	}

	if *fid == 0 && *fimport == "" {
		fmt.Println("Please provide an ID or a json file")
		os.Exit(1)
	}

	f, err := os.Create(*fout)
	if err != nil {
		fmt.Println("Error creating output file:", err)
		os.Exit(1)
	}
	defer f.Close()

	barQueue := progressbar.NewOptions(-1,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSpinnerType(25),
	)

	barTimeout := progressbar.NewOptions(60,
		progressbar.OptionSetDescription("Waiting for timeout"),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]━[reset]",
			SaucerHead:    "[green][reset]",
			SaucerPadding: "[red]━[reset]",
			BarStart:      "[",
			BarEnd:        "]",
		}))
	barTimeout.Clear()

	// TODO could do a .WithProgress() that combines the two bars, but this is fine for now
	err = malaina.CreateGraph(f, *fid, *fexport, *fimport, func(id, seen, queue int, err error) {
		if err != nil {
			if err.Error() == "Too Many Requests." {
				barTimeout.Reset()
			}

			if err.Error() == "Waiting for timeout" {
				barTimeout.Add(1)
			}
		} else {
			barQueue.Set(seen) // safer
			barQueue.Describe(fmt.Sprintf("Querying: %d (%d / %d)", id, seen, seen+queue))
		}
	})
	if err != nil {
		fmt.Println("Error creating graph:", err)
		os.Exit(1)
	}

	// might need file://
	err = openBrowser(*fout)
	if err != nil {
		fmt.Println("Error opening browser:", err)
		os.Exit(1)
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // "linux", "freebsd", etc.
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
}

func searchAnimeId(name string) (id int, err error) {
	media, err := malaina.SearchAnime(name)
	if err != nil {
		return id, errors.New("Error searching anime: " + err.Error())
	}

	var choices []Choice
	for _, m := range media {
		title := m.Title.English
		if title == "" {
			title = m.Title.Romaji
		}
		choices = append(choices, Choice{Id: m.Id, Title: title})
	}

	// search as a seperate call seems bad. they only ever need it for this
	// a picker would be cool, but increase size a lot
	// so a basic prompt is fine
	// actually it doesnt, and its cool
	p := tea.NewProgram(initialModel(choices))

	finalModel, err := p.Run()
	if err != nil {
		return id, errors.New("Error running prompt: " + err.Error())
	}

	m := finalModel.(model)
	if m.selected < 0 {
		return id, errors.New("no anime selected")
	}

	return choices[m.selected].Id, nil
}

type model struct {
	choices  []Choice
	cursor   int
	selected int
}

type Choice struct {
	Id    int
	Title string
}

func initialModel(choices []Choice) model {
	return model{
		choices:  choices,
		cursor:   0,
		selected: -1,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = m.cursor
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	s := "Select an anime to start from:\n\n"
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s\n", cursor, choice.Title)
	}
	return s
}
