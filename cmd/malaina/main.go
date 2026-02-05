//go:build !js || !wasm

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"malaina/internal"

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

func queryAnimes(aid int) (medias []Media, err error) {
	queue := []int{aid}
	seen := []int{}

	barQueue := progressbar.NewOptions(-1,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSpinnerType(25),
	)

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		barQueue.Set(len(seen)) // safer
		barQueue.Describe(fmt.Sprintf("Querying: %d (%d / %d)", id, len(seen), len(seen)+len(queue)))

		media, err := queryAnime(id)
		if err != nil {
			fmt.Println(id, ":", err)
			if err.Error() == "Too Many Requests." {
				queue = append(queue, id)
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
				for range 60 {
					barTimeout.Add(1)
					time.Sleep(1 * time.Second)
				}
			}
			continue
		}

		for _, related := range media.Relations.Edges {
			if related.Node.Id == 0 {
				continue
			}

			if !Contains(internal.RelatedTypes, related.RelationType, func(a, b string) bool {
				return a == b
			}) {
				continue
			}

			// dont even add known ones
			if Contains(seen, related.Node.Id, func(a, b int) bool {
				return a == b
			}) {
				continue
			}
			// also check if its in queue
			if Contains(queue, related.Node.Id, func(a, b int) bool {
				return a == b
			}) {
				continue
			}

			queue = append(queue, related.Node.Id)
		}

		seen = append(seen, id)
		medias = append(medias, media)
		time.Sleep(1 * time.Second)
	}

	return
}

type PageData struct {
	Title     string
	Medias    []Media
	Links     []Link
	Episodes  int
	WatchTime int
}

type Response struct {
	Data struct {
		Media Media `json:"Media"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type ResponseSearch struct {
	Data struct {
		Page struct {
			Media []Media `json:"media"`
		} `json:"Page"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// TODO more data
type Media struct {
	Id       int    `json:"id"`
	Episodes int    `json:"episodes"`
	Duration int    `json:"duration"`
	Format   string `json:"format"`
	Title    struct {
		English string `json:"english"`
		Romaji  string `json:"romaji"`
	} `json:"title"`
	Relations struct {
		Edges []struct {
			RelationType string `json:"relationType"`
			Node         struct {
				Id int `json:"id"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"relations"`
	CoverImage struct {
		Medium string `json:"medium"`
	} `json:"coverImage"`
}

type Link struct {
	To       int
	From     int
	Relation string
}

func queryAnime(id int) (media Media, err error) {
	// Define the GraphQL query
	query := map[string]string{
		// format_in: [ TV, TV_SHORT, MOVIE, SPECIAL, OVA, ONA ]
		// ignoring manga hides some anime
		"query": fmt.Sprintf(`
			{
				Media(id: %d) {
					id,
					episodes,
					duration,
					format,
					title {
						english,
						romaji
					},
					relations {
						edges {
							relationType,
							node {
								id
							}
						}
					},
					coverImage {
						medium
					}
				}
			}
		`, id),
	}
	jsonValue, _ := json.Marshal(query)

	// TODO mal id can be wrong (4081 -> 1859)
	// maybe check the name is the same
	// nah, the backend doesnt matter, and dont want user to confirm
	// might be manga to anime adaptation

	// TODO add score and link

	// Send the HTTP request
	request, _ := http.NewRequest("POST", "https://graphql.anilist.co", bytes.NewBuffer(jsonValue))
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()

	// Parse the response
	data, _ := io.ReadAll(response.Body)
	var res Response
	if err := json.Unmarshal(data, &res); err != nil {
		return media, err
	}

	if len(res.Errors) > 0 {
		return media, errors.New(res.Errors[0].Message)
	}

	return res.Data.Media, nil
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

func Contains[T any](slice []T, value T, comp func(T, T) bool) bool {
	for _, item := range slice {
		if comp(item, value) {
			return true
		}
	}
	return false
}

func searchAnimeId(name string) (id int, err error) {
	media, err := searchAnime(name)
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

func searchAnime(name string) (media []Media, err error) {
	query := map[string]string{
		// format_in: [ TV, TV_SHORT, MOVIE, SPECIAL, OVA, ONA ]
		// ignoring manga hides some anime
		"query": fmt.Sprintf(`
			{
				Page {
					media(search: "%s", type: ANIME) {
						id
						title {
							english
							romaji
						}
					}
				}
			}
		`, name),
	}
	jsonValue, _ := json.Marshal(query)

	// Send the HTTP request
	request, _ := http.NewRequest("POST", "https://graphql.anilist.co", bytes.NewBuffer(jsonValue))
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()

	// Parse the response
	data, _ := io.ReadAll(response.Body)
	var res ResponseSearch
	if err := json.Unmarshal(data, &res); err != nil {
		return media, err
	}

	if len(res.Errors) > 0 {
		return media, errors.New(res.Errors[0].Message)
	}

	return res.Data.Page.Media, nil
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

/*
{
  show(id: "1") {
    ...ShowWithSequels
  }
}

fragment ShowWithSequels on Show {
  title
  sequel {
    ...ShowWithSequels
  }
}
*/
