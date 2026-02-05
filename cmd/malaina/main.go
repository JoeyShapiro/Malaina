//go:build !js || !wasm

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"runtime"

	"malaina/internal"

	tea "github.com/charmbracelet/bubbletea"
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

	var medias []Media
	if *fid != 0 {
		medias, err = queryAnimes(*fid)
		if err != nil {
			fmt.Println("Error querying anime:", err)
			os.Exit(1)
		}

		// write medias to json file
		if *fexport != "" {
			data, _ := json.Marshal(medias)
			os.WriteFile(*fexport, data, 0644)
		}
	} else {
		content, err := os.ReadFile(*fimport)
		if err != nil {
			fmt.Println("Error reading file:", err)
			os.Exit(1)
		}
		if err := json.Unmarshal(content, &medias); err != nil {
			fmt.Println("Error parsing json:", err)
			os.Exit(1)
		}
	}

	// // write medias to json file
	// data, _ := json.Marshal(medias)
	// os.WriteFile("medias.json", data, 0644)

	watchTime := 0
	episodes := 0
	var links []Link

	// do stuff with it separately
	for _, media := range medias {
		watchTime += media.Duration * media.Episodes
		episodes += media.Episodes

		for _, related := range media.Relations.Edges {
			// TODO handle empty ones
			if !Contains(medias, Media{Id: related.Node.Id}, func(a, b Media) bool {
				return a.Id == b.Id
			}) {
				continue
			}

			links = append(links, Link{
				media.Id,
				related.Node.Id,
				related.RelationType})
		}
	}

	// Create some data to pass to the template
	page := PageData{
		Title:     medias[0].Title.English,
		Medias:    medias,
		Links:     links,
		Episodes:  episodes,
		WatchTime: watchTime,
	}

	// Parse the template file
	tmpl, err := template.ParseFS(internal.TemplateFS, "template.html")
	if err != nil {
		fmt.Println("Error parsing template:", err)
		os.Exit(1)
	}

	// Execute the template with our data
	f, err := os.Create(*fout)
	if err != nil {
		fmt.Println("Error creating output file:", err)
		os.Exit(1)
	}
	defer f.Close()
	err = tmpl.ExecuteTemplate(f, "template.html", page)
	if err != nil {
		fmt.Println("Error executing template:", err)
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
