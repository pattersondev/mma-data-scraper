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

type Fighter struct {
	Name         string
	Stats        []FightStats
	Bio          FighterBio
	FightHistory []FightHistoryEntry
}

type FighterBio struct {
	Country   string
	WTClass   string
	HTWT      string
	Birthdate string
	Team      string
	Nickname  string
	Stance    string
	Reach     string
}

type FightStats struct {
	Date     string
	Opponent string
	Event    string
	Result   string
	Striking StrikingStats
	Clinch   ClinchStats
	Ground   GroundStats
}

type StrikingStats struct {
	SDBL_A   string
	SDHL_A   string
	SDLL_A   string
	TSL      string
	TSA      string
	SSL      string
	SSA      string
	TSL_TSA  string
	KD       string
	BodyPerc string
	HeadPerc string
	LegPerc  string
}

type ClinchStats struct {
	// Add clinch-specific fields here
}

type GroundStats struct {
	// Add ground-specific fields here
}

type FightHistoryEntry struct {
	Date     string
	Opponent string
	Result   string
	Decision string
	Round    string
	Time     string
	Event    string
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
		go worker(c, urlChan, &wg, visitedURLs, &mu)
	}

	// Send initial URL
	urlChan <- "https://www.espn.com/mma/"

	// Close channel when all URLs have been processed
	go func() {
		wg.Wait()
		close(urlChan)
	}()

	// Wait for all goroutines to finish
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
		// currentURL := e.Request.URL.String()
		// if strings.Contains(currentURL, "fighter/stats") {
		// 	handleFighterStats(e, fighterMap)
		// }
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

func handleFighterData(e *colly.HTMLElement, fighterMap map[string]*Fighter) {
	fighterName := e.ChildText("h1")
	fmt.Println("Processing fighter data for:", fighterName)
	fmt.Println("Current URL:", e.Request.URL.String())

	// currentFighter := getOrCreateFighter(fighterMap, fighterName)

	// Check if it's a bio page
	if strings.Contains(e.Request.URL.String(), "bio") {
		fmt.Println("Processing bio for:", fighterName)
		handleFighterBio(e, fighterMap)
	}

	// Check if it's a fight history page
	if e.ChildText("div.ResponsiveTable.fight-history") != "" {
		fmt.Println("Processing fight history for:", fighterName)
		handleFightHistory(e, fighterMap)
	}

	// You can add more checks here for other types of fighter data
}

func getOrCreateFighter(fighterMap map[string]*Fighter, fighterName string) *Fighter {
	currentFighter, exists := fighterMap[fighterName]
	if !exists {
		currentFighter = &Fighter{
			Name:         fighterName,
			Stats:        []FightStats{},
			Bio:          FighterBio{},
			FightHistory: []FightHistoryEntry{},
		}
		fighterMap[fighterName] = currentFighter
	}
	return currentFighter
}

func handleFighterStats(e *colly.HTMLElement, fighterMap map[string]*Fighter) {
	fighterName := e.ChildText("h1")
	fmt.Printf("Processing stats for fighter: %s\n", fighterName)
	currentFighter := getOrCreateFighter(fighterMap, fighterName)

	statsFound := false
	tableSelector := "#fittPageContainer > div.pageContent > div > div:nth-child(5) > div > div.PageLayout__Main > div > section > div > div:nth-child(2) > div.flex > div > div.Table__Scroller > table"

	e.ForEach(tableSelector, func(_ int, table *colly.HTMLElement) {
		fmt.Println("Found the target table")
		newStats := extractStatsFromTable(table)
		if len(newStats) > 0 {
			currentFighter.Stats = append(currentFighter.Stats, newStats...)
			statsFound = true
			fmt.Printf("Extracted %d striking stats\n", len(newStats))
		} else {
			fmt.Println("No striking stats extracted")
		}
	})

	if !statsFound {
		fmt.Printf("No stats found for fighter: %s\n", fighterName)
	} else {
		fmt.Printf("Updated stats for fighter: %s\n", fighterName)
	}
}

func extractStatsFromTable(table *colly.HTMLElement) []FightStats {
	var stats []FightStats

	table.ForEach("tbody tr.Table__TR.Table__TR--sm", func(_ int, row *colly.HTMLElement) {
		var cells []string
		row.ForEach("td.Table__TD", func(_ int, cell *colly.HTMLElement) {
			cells = append(cells, cell.Text)
		})

		if len(cells) >= 16 {
			stat := FightStats{
				Date:     strings.TrimSpace(cells[0]),
				Opponent: cleanFighterName(cells[1]),
				Event:    strings.TrimSpace(cells[2]),
				Result:   strings.TrimSpace(strings.Split(cells[3], " ")[0]),
				Striking: StrikingStats{
					SDBL_A:   strings.TrimSpace(cells[4]),
					SDHL_A:   strings.TrimSpace(cells[5]),
					SDLL_A:   strings.TrimSpace(cells[6]),
					TSL:      strings.TrimSpace(cells[7]),
					TSA:      strings.TrimSpace(cells[8]),
					SSL:      strings.TrimSpace(cells[9]),
					SSA:      strings.TrimSpace(cells[10]),
					TSL_TSA:  strings.TrimSpace(cells[11]),
					KD:       strings.TrimSpace(cells[12]),
					BodyPerc: strings.TrimSpace(cells[13]),
					HeadPerc: strings.TrimSpace(cells[14]),
					LegPerc:  strings.TrimSpace(cells[15]),
				},
			}

			// Handle cases where stats are not available
			if stat.Striking.SDBL_A == "-" {
				stat.Striking = StrikingStats{}
			}

			stats = append(stats, stat)
			fmt.Printf("Extracted stat: %+v\n", stat)
		}
	})

	fmt.Printf("Total rows processed: %d\n", len(stats))
	return stats
}

func handleFighterBio(e *colly.HTMLElement, fighterMap map[string]*Fighter) {
	fighterName := e.ChildText("h1")
	fmt.Println("Processing bio for fighter:", fighterName)
	currentFighter := getOrCreateFighter(fighterMap, fighterName)
	e.ForEach("div.Bio__Item", func(_ int, item *colly.HTMLElement) {
		label := item.ChildText("span.Bio__Label")
		value := item.ChildText("span.clr-gray-01")
		updateFighterBio(&currentFighter.Bio, label, value)
	})
	fmt.Printf("Updated bio for fighter: %s\n", currentFighter.Name)
}

func updateFighterBio(bio *FighterBio, label, value string) {
	switch label {
	case "Country":
		bio.Country = value
	case "WT Class":
		bio.WTClass = value
	case "HT/WT":
		bio.HTWT = value
	case "Birthdate":
		bio.Birthdate = value
	case "Team":
		bio.Team = value
	case "Nickname":
		bio.Nickname = value
	case "Stance":
		bio.Stance = value
	case "Reach":
		bio.Reach = value
	}
}

func handleFightHistory(e *colly.HTMLElement, fighterMap map[string]*Fighter) {
	fighterName := e.ChildText("h1")
	fmt.Println("Processing fight history for:", fighterName)
	currentFighter := getOrCreateFighter(fighterMap, fighterName)

	e.ForEach("tr.Table__TR", func(_ int, row *colly.HTMLElement) {
		entry := extractFightHistoryEntry(row)
		currentFighter.FightHistory = append(currentFighter.FightHistory, entry)
	})

	fmt.Printf("Updated fight history for fighter: %s\n", currentFighter.Name)
}

func extractFightHistoryEntry(row *colly.HTMLElement) FightHistoryEntry {
	return FightHistoryEntry{
		Date:     row.ChildText("td:nth-child(1)"),
		Opponent: row.ChildText("td:nth-child(2)"),
		Result:   row.ChildText("td:nth-child(3)"),
		Decision: row.ChildText("td:nth-child(4)"),
		Round:    row.ChildText("td:nth-child(5)"),
		Time:     row.ChildText("td:nth-child(6)"),
		Event:    row.ChildText("td:nth-child(7)"),
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

func writeFighterDataToJSON(fighterMap map[string]*Fighter) {
	jsonFileName := fmt.Sprintf("fighters%s.json", time.Now().Format("2006-01-02_15-04-05"))
	file, err := os.Create(jsonFileName)
	if err != nil {
		log.Fatal("Cannot create file", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(fighterMap); err != nil {
		log.Fatal("Error writing to JSON:", err)
	}

	fmt.Printf("Fighter data JSON file created: %s\n", jsonFileName)
}
