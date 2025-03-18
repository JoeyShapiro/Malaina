package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	var medias []Media
	queue := []int{4081}
	seen := []int{}
	watchTime := 0
	episodes := 0

loop_queue:
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		for _, idSeen := range seen {
			if id == idSeen {
				continue loop_queue
			}
		}
		seen = append(seen, id)

		media, err := queryAnime(id)
		if err != nil {
			fmt.Println(id, ":", err)
			if err.Error() == "Too Many Requests." {
				queue = append(queue, id)
				time.Sleep(60 * time.Second)
			}
			continue
		}

		watchTime += media.Duration * media.Episodes
		episodes += media.Episodes

		for _, related := range media.Relations.Edges {
			queue = append(queue, related.Node.Id)
		}

		fmt.Printf("%+v\n", media)
		medias = append(medias, media)
	}

	fmt.Println("Total watch time:", watchTime/60, "hours")
	fmt.Println("Total episodes:", episodes)

	// write medias to json file
	data, _ := json.Marshal(medias)
	os.WriteFile("medias.json", data, 0644)
}

type Response struct {
	Data struct {
		Media Media `json:"Media"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type Media struct {
	Id       int    `json:"idMal"`
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
}

func queryAnime(id int) (media Media, err error) {
	// Define the GraphQL query
	query := map[string]string{
		// format_in: [ TV, TV_SHORT, MOVIE, SPECIAL, OVA, ONA ]
		// ignoring manga hides some anime
		"query": fmt.Sprintf(`
			{
				Media(idMal: %d) {
					idMal,
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
					}
				}
			}
		`, id),
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
	var res Response
	if err := json.Unmarshal(data, &res); err != nil {
		return media, err
	}

	if len(res.Errors) > 0 {
		return media, fmt.Errorf(res.Errors[0].Message)
	}

	return res.Data.Media, nil
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
