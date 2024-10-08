package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

type Event struct {
	Name     string
	Date     string
	Location string
	Matchups []FightData
}

type FightData struct {
	Fighter1 string
	Fighter2 string
	Result   string
	Winner   string
}

func main() {
	c := initializeCollector()
	events := scrapeData(c)

	fmt.Printf("Total events found: %d\n", len(events))

	writeEventDataToJSON(events)
}

func initializeCollector() *colly.Collector {
	c := colly.NewCollector(
		colly.AllowedDomains("www.espn.com"),
		colly.MaxDepth(3),
	)

	// Add rate limiting
	err := c.Limit(&colly.LimitRule{
		DomainGlob: "*espn.com*",
		// Delay:       2 * time.Second, // 2 seconds delay between requests
		// RandomDelay: 1 * time.Second, // Add up to 1 second of random delay
	})
	if err != nil {
		log.Fatalf("Failed to set rate limit: %v", err)
	}

	return c
}

// func scrapeData(c *colly.Collector) []Event {
// 	var events []Event
// 	var mu sync.Mutex
// 	visitedURLs := make(map[string]bool)
// 	urlChan := make(chan string, 100)
// 	var wg sync.WaitGroup

// 	setupCollectorCallbacks(c, &events, &mu, visitedURLs)

// 	// Start worker goroutines
// 	for i := 0; i < 3; i++ { // Adjust the number of workers as needed
// 		wg.Add(1)
// 		go worker(c, urlChan, &wg, visitedURLs, &mu)
// 	}

// 	// Send initial URL
// 	urlChan <- "https://www.espn.com/mma/"

// 	// Wait for all goroutines to finish
// 	wg.Wait()

// 	// Close channel after all workers are done
// 	close(urlChan)

// 	return events
// }

func scrapeData(c *colly.Collector) []Event {
	var events []Event
	var mu sync.Mutex
	visitedURLs := make(map[string]bool)
	urlChan := make(chan string, 100)
	var wg sync.WaitGroup

	setupCollectorCallbacks(c, &events, &mu, visitedURLs)

	// Start worker goroutines
	for i := 0; i < 3; i++ { // Adjust the number of workers as needed
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(c, urlChan, &wg, visitedURLs, &mu)
		}()
	}

	// Send initial URL
	urlChan <- "https://www.espn.com/mma/"

	// Close channel after all URLs have been sent
	go func() {
		wg.Wait()      // Wait for all workers to finish
		close(urlChan) // Then close the channel
	}()

	// Wait for all goroutines to finish before returning
	wg.Wait()

	return events
}

func worker(c *colly.Collector, urlChan chan string, wg *sync.WaitGroup, visitedURLs map[string]bool, mu *sync.Mutex) {
	defer wg.Done()

	for url := range urlChan {
		mu.Lock()
		if visitedURLs[url] {
			mu.Unlock()
			continue
		}
		visitedURLs[url] = true
		mu.Unlock()

		err := c.Visit(url)
		if err != nil {
			fmt.Printf("Error visiting %s: %v\n", url, err)
		}
	}
}

func setupCollectorCallbacks(c *colly.Collector, events *[]Event, mu *sync.Mutex, visitedURLs map[string]bool) {
	// fighterMap := make(map[string]*Fighter)

	c.OnRequest(func(r *colly.Request) {
		handleRequest(r)
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		handleLinks(e, c, visitedURLs, mu)
	})

	c.OnHTML("body", func(e *colly.HTMLElement) {
		currentURL := e.Request.URL.String()
		if strings.Contains(currentURL, "fightcenter") {
			event := extractEventData(e)
			mu.Lock()
			*events = append(*events, event)
			mu.Unlock()
			printEventInfo(event)
		} else {
			fmt.Println("Unhandled page type:", currentURL)
			if currentURL == "https://www.espn.com/mma/" {
				//Close channels, free up workers,
				writeEventDataToJSON(*events)
				os.Exit(0)
			}
		}
	})
}

func handleLinks(e *colly.HTMLElement, c *colly.Collector, visitedURLs map[string]bool, mu *sync.Mutex) {
	link := e.Attr("href")
	absoluteURL := e.Request.AbsoluteURL(link)
	if shouldVisitURL(absoluteURL) {
		mu.Lock()
		if !visitedURLs[absoluteURL] {
			visitedURLs[absoluteURL] = true
			mu.Unlock()
			fmt.Println("Queuing", absoluteURL)
			c.Visit(absoluteURL)
		} else {
			mu.Unlock()
		}
	}
}

func shouldVisitURL(url string) bool {
	return (strings.Contains(url, "espn.com/mma/fightcenter") ||
		strings.Contains(url, "espn.com/mma/fighter/")) &&
		!strings.Contains(url, "news") && !strings.Contains(url, "stats") && !strings.Contains(url, "history") && !strings.Contains(url, "bio")
}

func extractEventData(e *colly.HTMLElement) Event {
	fmt.Println("Extracting event data from:", e.Request.URL.String())

	eventName := e.ChildText(".headline.headline__h1.mb3")
	if eventName == "" {
		eventName = e.ChildText("h1.headline") // Alternative selector
	}

	eventDate := e.ChildText(".n6.mb2")
	if eventDate == "" {
		eventDate = e.ChildText(".n6") // Alternative selector
	}
	eventDate = extractDateOnly(eventDate)

	eventLocation := e.ChildText("div.n8.clr-gray-04")
	if eventLocation == "" {
		eventLocation = e.ChildText(".n8") // Alternative selector
	}
	eventLocation = extractLocationOnly(eventLocation)

	event := Event{
		Name:     eventName,
		Date:     eventDate,
		Location: eventLocation,
		Matchups: []FightData{},
	}

	e.ForEach("div.MMAGamestrip", func(_ int, el *colly.HTMLElement) {
		fighter1 := cleanFighterName(el.ChildText("div.MMACompetitor:first-child h2"))
		fighter2 := cleanFighterName(el.ChildText("div.MMACompetitor:last-child h2"))

		result := el.ChildText("div.Gamestrip__Overview .ScoreCell__Time--post")
		cleanedResult := cleanResult(result)

		var winner string
		if cleanedResult == "" {
			winner = ""
		} else if el.ChildAttr("svg.MMACompetitor__arrow", "class") != "" {
			if strings.Contains(el.ChildAttr("svg.MMACompetitor__arrow", "class"), "--reverse") {
				winner = fighter1
			} else {
				winner = fighter2
			}
		} else {
			winner = "Draw/No Contest"
		}

		if fighter1 != "" && fighter2 != "" && fighter1 != fighter2 {
			fight := FightData{
				Fighter1: fighter1,
				Fighter2: fighter2,
				Result:   cleanedResult,
				Winner:   winner,
			}
			event.Matchups = append(event.Matchups, fight)
		}
	})

	return event
}

func cleanFighterName(name string) string {
	// Remove any numbers (usually record) from the name
	name = regexp.MustCompile(`\d+-\d+-\d+`).ReplaceAllString(name, "")

	// Remove any text in parentheses
	name = regexp.MustCompile(`\(.*?\)`).ReplaceAllString(name, "")

	// Split the name by spaces
	parts := strings.Fields(name)

	// Take the first two parts (assuming they are the first and last name)
	if len(parts) >= 2 {
		return strings.Join(parts[:2], " ")
	}

	return strings.TrimSpace(name)
}

func cleanResult(result string) string {
	result = strings.TrimSpace(result)
	if strings.Contains(strings.ToLower(result), "ppv") || strings.Contains(strings.ToLower(result), "espn+") {
		return "" // Return empty string for future fights
	}

	// Keep only the first part of the result (e.g., "FinalKO/TKOR1, 0:21")
	parts := strings.SplitN(result, "Final", 2)
	if len(parts) > 1 {
		return "Final" + strings.SplitN(parts[1], "Final", 2)[0]
	}
	return result
}

// Update this function to include the year
func extractDateOnly(fullText string) string {
	// Assuming the date is always at the beginning and in the format "Month Day, Year"
	dateParts := strings.SplitN(fullText, ",", 3)
	if len(dateParts) >= 2 {
		// Combine the month/day with the year
		return strings.TrimSpace(dateParts[0] + "," + dateParts[1])
	}
	return ""
}

func extractLocationOnly(fullText string) string {
	// List of keywords that typically appear after the location
	keywords := []string{"Final", "PPV", "ESPN+", "ESPN", "FOX", "FS1", "FS2", "Max"}

	// Find the first occurrence of any keyword
	index := len(fullText)
	for _, keyword := range keywords {
		if idx := strings.Index(fullText, keyword); idx != -1 && idx < index {
			index = idx
		}
	}

	// Extract the substring before the first keyword
	location := fullText[:index]

	// Remove any trailing commas and whitespace
	location = strings.TrimRight(location, ", ")

	return strings.TrimSpace(location)
}

func printEventInfo(event Event) {
	fmt.Printf("Event: %s, Date: %s, Location: %s\n", event.Name, event.Date, event.Location)
	fmt.Printf("Total matchups: %d\n", len(event.Matchups))
	for _, matchup := range event.Matchups {
		fmt.Printf("  %s vs %s - Result: %s, Winner: %s\n", matchup.Fighter1, matchup.Fighter2, matchup.Result, matchup.Winner)
	}
}

func handleRequest(r *colly.Request) {
	fmt.Println("Attempting to visit:", r.URL.String())
	if strings.Contains(r.URL.String(), "radio") ||
		strings.Contains(r.URL.String(), "watch") ||
		strings.Contains(r.URL.String(), "news") {
		r.Abort()
		fmt.Println("Skipping", r.URL.String())
	} else {
		fmt.Println("Visiting", r.URL.String())
	}
}

func writeEventDataToJSON(events []Event) {
	jsonFileName := fmt.Sprintf("events%s.json", time.Now().Format("2006-01-02_15-04-05"))
	file, err := os.Create(jsonFileName)
	if err != nil {
		log.Fatal("Cannot create file", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(events); err != nil {
		log.Fatal("Error writing to JSON:", err)
	}

	fmt.Printf("Event data JSON file created: %s\n", jsonFileName)
}
