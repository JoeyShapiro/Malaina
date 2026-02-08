package malaina

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"time"
)

//go:embed template.html
var TemplateFS embed.FS
var RelatedTypes = []string{
	"SEQUEL", "PREQUEL",
	"PARENT", "SIDE_STORY",
	"ALTERNATIVE", "SPIN_OFF",
	"SUMMARY",
}

func CreateGraph(wr io.Writer, anime int, fexport string, fimport string, progress func(id int, seen int, queue int, err error)) (err error) {
	var medias []Media
	if anime != 0 {
		medias, err = queryAnimes(anime, progress)
		if err != nil {
			return errors.New("error querying anime: " + err.Error())
		}

		// write medias to json file
		if fexport != "" {
			data, _ := json.Marshal(medias)
			os.WriteFile(fexport, data, 0644)
		}
	} else {
		content, err := os.ReadFile(fimport)
		if err != nil {
			return errors.New("error reading file: " + err.Error())
		}
		if err := json.Unmarshal(content, &medias); err != nil {
			return errors.New("error parsing json: " + err.Error())
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
	tmpl, err := template.ParseFS(TemplateFS, "template.html")
	if err != nil {
		return errors.New("error parsing template: " + err.Error())
	}

	// Execute the template with our data
	err = tmpl.ExecuteTemplate(wr, "template.html", page)
	if err != nil {
		return errors.New("error executing template: " + err.Error())
	}

	return nil
}

func queryAnimes(aid int, progress func(id int, seen int, queue int, err error)) (medias []Media, err error) {
	queue := []int{aid}
	seen := []int{}

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		if progress != nil {
			progress(id, len(seen), len(queue), nil)
		}

		media, err := queryAnime(id)
		if err != nil {
			fmt.Println(id, ":", err)
			if progress != nil {
				progress(id, len(seen), len(queue), err)
			}
			if err.Error() == "Too Many Requests." {
				queue = append(queue, id)

				for range 60 {
					if progress != nil {
						progress(id, len(seen), len(queue), errors.New("Waiting for timeout"))
					}
					time.Sleep(1 * time.Second)
				}
			}
			continue
		}

		for _, related := range media.Relations.Edges {
			if related.Node.Id == 0 {
				continue
			}

			if !Contains(RelatedTypes, related.RelationType, func(a, b string) bool {
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

type ResponseAnime struct {
	Data struct {
		Media Media `json:"Media"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type ResponseSearch struct {
	Data struct { // could maybe do Data T, but not really the golang style
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

func queryAnilist(query map[string]string) (data []byte, err error) {
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

	return io.ReadAll(response.Body)
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

	data, err := queryAnilist(query)
	if err != nil {
		return media, err
	}

	// Parse the response
	var res ResponseAnime
	if err := json.Unmarshal(data, &res); err != nil {
		return media, err
	}

	if len(res.Errors) > 0 {
		return media, errors.New(res.Errors[0].Message)
	}

	return res.Data.Media, nil
}

func SearchAnime(name string) (media []Media, err error) {
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

	data, err := queryAnilist(query)
	if err != nil {
		return media, err
	}

	// Parse the response
	var res ResponseSearch
	if err := json.Unmarshal(data, &res); err != nil {
		return media, err
	}

	if len(res.Errors) > 0 {
		return media, errors.New(res.Errors[0].Message)
	}

	return res.Data.Page.Media, nil
}

func Contains[T any](slice []T, value T, comp func(T, T) bool) bool {
	for _, item := range slice {
		if comp(item, value) {
			return true
		}
	}
	return false
}
