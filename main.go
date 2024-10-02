package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

type FightData struct {
	EventName     string
	EventDate     string
	EventLocation string
	Fighter1      string
	Fighter2      string
	Result        string
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
	allFights, fighterMap := scrapeData(c)

	fmt.Printf("Total fights found: %d\n", len(allFights))

	writeEventDataToCSV(allFights)
	writeFighterDataToCSV(fighterMap)
}

func initializeCollector() *colly.Collector {
	return colly.NewCollector(
		colly.AllowedDomains("www.espn.com"),
		colly.MaxDepth(3),
	)
}

func scrapeData(c *colly.Collector) ([]FightData, map[string]*Fighter) {
	var allFights []FightData
	fighterMap := make(map[string]*Fighter)
	visitedURLs := make(map[string]bool)

	setupCollectorCallbacks(c, &allFights, fighterMap, visitedURLs)

	err := c.Visit("https://www.espn.com/mma/fightcenter")
	if err != nil {
		log.Fatal(err)
	}

	return allFights, fighterMap
}

func setupCollectorCallbacks(c *colly.Collector, allFights *[]FightData, fighterMap map[string]*Fighter, visitedURLs map[string]bool) {
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		handleLinks(e, c, visitedURLs)
	})

	c.OnHTML(".ResponsiveWrapper", func(e *colly.HTMLElement) {
		handleFightData(e, allFights)
	})

	c.OnHTML("div.Wrapper.Card__Content", func(e *colly.HTMLElement) {
		fmt.Println("Handling fighter data for URL:", e.Request.URL.String())
		handleFighterData(e, fighterMap)
	})

	c.OnHTML("div.ResponsiveTable.fight-history", func(e *colly.HTMLElement) {
		fmt.Println("Handling fight history for URL:", e.Request.URL.String())
		handleFightHistory(e, fighterMap)
	})

	c.OnHTML("div.ResponsiveTable.fighter-stats", func(e *colly.HTMLElement) {
		fmt.Println("Stats page detected:", e.Request.URL.String())
		handleFighterStats(e, fighterMap)
	})

	c.OnHTML("div.PlayerBio", func(e *colly.HTMLElement) {
		fmt.Println("Bio page detected:", e.Request.URL.String())
		handleFighterBio(e, fighterMap)
	})

	c.OnRequest(func(r *colly.Request) {
		handleRequest(r)
	})
}

func handleLinks(e *colly.HTMLElement, c *colly.Collector, visitedURLs map[string]bool) {
	link := e.Attr("href")
	absoluteURL := e.Request.AbsoluteURL(link)
	if !visitedURLs[absoluteURL] &&
		(strings.Contains(absoluteURL, "espn.com/mma/fightcenter") ||
			strings.Contains(absoluteURL, "espn.com/mma/fight") ||
			strings.Contains(absoluteURL, "espn.com/mma/fighter")) &&
		!strings.Contains(absoluteURL, "news") {
		visitedURLs[absoluteURL] = true
		fmt.Println("Queuing", absoluteURL)
		err := c.Visit(absoluteURL)
		if err != nil {
			fmt.Printf("Error visiting %s: %v\n", absoluteURL, err)
		}
	}
}

func handleFightData(e *colly.HTMLElement, allFights *[]FightData) {
	fight := extractFightData(e)
	*allFights = append(*allFights, fight)
	printFightInfo(fight)
}

func extractFightData(e *colly.HTMLElement) FightData {
	fight := FightData{
		EventName:     e.ChildText(".MMAFightCard__GameNote"),
		Fighter1:      e.ChildText(".MMACompetitor:first-child .MMACompetitor__Detail h2"),
		Fighter2:      e.ChildText(".MMACompetitor:last-child .MMACompetitor__Detail h2"),
		Result:        e.ChildText(".Gamestrip__Overview .ScoreCell__Time--post h3"),
		EventDate:     e.ChildText(".Gamestrip__Overview .ScoreCell__Time--post .n9"),
		EventLocation: e.ChildText(".MMAEventHeader__Event .n8.clr-gray-04"),
	}

	e.ForEach(".Gamestrip__Overview .ScoreCell__Time--post div", func(_ int, el *colly.HTMLElement) {
		if el.Index == 1 {
			fight.Result += " - " + el.Text
		}
	})

	return fight
}

func printFightInfo(fight FightData) {
	fmt.Printf("Fight found: %s vs %s - Result: %s\nEvent: %s, Date: %s, Location: %s\n",
		fight.Fighter1, fight.Fighter2, fight.Result, fight.EventName, fight.EventDate, fight.EventLocation)
}

func handleFighterData(e *colly.HTMLElement, fighterMap map[string]*Fighter) {
	fighterName := e.ChildText("h1")
	fmt.Println("Processing fighter data for:", fighterName)

	// Check if it's a stats page
	if e.DOM.Find("div.ResponsiveTable.fighter-stats").Length() > 0 {
		fmt.Println("Processing stats for:", fighterName)
		handleFighterStats(e, fighterMap)
	}

	// Check if it's a bio page
	if e.DOM.Find("div.PlayerBio").Length() > 0 {
		fmt.Println("Processing bio for:", fighterName)
		handleFighterBio(e, fighterMap)
	}

	// Check if it's a fight history page
	if e.DOM.Find("div.ResponsiveTable.fight-history").Length() > 0 {
		fmt.Println("Processing fight history for:", fighterName)
		handleFightHistory(e, fighterMap)
	}
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
	fmt.Println("Processing stats for fighter:", fighterName)
	currentFighter := getOrCreateFighter(fighterMap, fighterName)
	e.ForEach("tr.Table__TR", func(_ int, row *colly.HTMLElement) {
		stats := extractFightStats(row)
		currentFighter.Stats = append(currentFighter.Stats, stats)
	})
	fmt.Printf("Updated stats for fighter: %s\n", currentFighter.Name)
}

func extractFightStats(row *colly.HTMLElement) FightStats {
	return FightStats{
		Date:     row.ChildText("td:nth-child(1)"),
		Opponent: row.ChildText("td:nth-child(2)"),
		Event:    row.ChildText("td:nth-child(3)"),
		Result:   row.ChildText("td:nth-child(4)"),
		SDBL_A:   row.ChildText("td:nth-child(5)"),
		SDHL_A:   row.ChildText("td:nth-child(6)"),
		SDLL_A:   row.ChildText("td:nth-child(7)"),
		TSL:      row.ChildText("td:nth-child(8)"),
		TSA:      row.ChildText("td:nth-child(9)"),
		SSL:      row.ChildText("td:nth-child(10)"),
		SSA:      row.ChildText("td:nth-child(11)"),
		TSL_TSA:  row.ChildText("td:nth-child(12)"),
		KD:       row.ChildText("td:nth-child(13)"),
		BodyPerc: row.ChildText("td:nth-child(14)"),
		HeadPerc: row.ChildText("td:nth-child(15)"),
		LegPerc:  row.ChildText("td:nth-child(16)"),
	}
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

func writeEventDataToCSV(allFights []FightData) {
	csvFileName := fmt.Sprintf("espn_mma_fights_%s.csv", time.Now().Format("2006-01-02_15-04-05"))
	file, err := os.Create(csvFileName)
	if err != nil {
		log.Fatal("Cannot create file", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"Event Name", "Event Date", "Event Location", "Fighter 1", "Fighter 2", "Result"}
	if err := writer.Write(header); err != nil {
		log.Fatal("Error writing header to CSV:", err)
	}

	for _, fight := range allFights {
		record := []string{
			fight.EventName,
			fight.EventDate,
			fight.EventLocation,
			fight.Fighter1,
			fight.Fighter2,
			fight.Result,
		}
		if err := writer.Write(record); err != nil {
			log.Fatal("Error writing record to CSV:", err)
		}
	}

	fmt.Printf("Event data CSV file created: %s\n", csvFileName)
}

func writeFighterDataToCSV(fighterMap map[string]*Fighter) {
	csvFileName := fmt.Sprintf("espn_mma_fighters_%s.csv", time.Now().Format("2006-01-02_15-04-05"))
	file, err := os.Create(csvFileName)
	if err != nil {
		log.Fatal("Cannot create file", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{
		"Name", "Country", "Weight Class", "Height/Weight", "Birthdate", "Team", "Nickname", "Stance", "Reach",
		"Stats_Date", "Stats_Opponent", "Stats_Event", "Stats_Result", "Stats_SDBL_A", "Stats_SDHL_A", "Stats_SDLL_A",
		"Stats_TSL", "Stats_TSA", "Stats_SSL", "Stats_SSA", "Stats_TSL_TSA", "Stats_KD", "Stats_BodyPerc", "Stats_HeadPerc", "Stats_LegPerc",
		"History_Date", "History_Opponent", "History_Result", "History_Decision", "History_Round", "History_Time", "History_Event",
	}
	if err := writer.Write(header); err != nil {
		log.Fatal("Error writing header to CSV:", err)
	}

	for _, fighter := range fighterMap {
		// Determine the maximum number of entries (either stats or fight history)
		maxEntries := max(len(fighter.Stats), len(fighter.FightHistory))

		for i := 0; i < maxEntries; i++ {
			record := []string{
				fighter.Name,
				fighter.Bio.Country,
				fighter.Bio.WTClass,
				fighter.Bio.HTWT,
				fighter.Bio.Birthdate,
				fighter.Bio.Team,
				fighter.Bio.Nickname,
				fighter.Bio.Stance,
				fighter.Bio.Reach,
			}

			// Add Stats data if available
			if i < len(fighter.Stats) {
				stats := fighter.Stats[i]
				record = append(record,
					stats.Date, stats.Opponent, stats.Event, stats.Result,
					stats.SDBL_A, stats.SDHL_A, stats.SDLL_A, stats.TSL,
					stats.TSA, stats.SSL, stats.SSA, stats.TSL_TSA,
					stats.KD, stats.BodyPerc, stats.HeadPerc, stats.LegPerc,
				)
			} else {
				record = append(record, make([]string, 16)...) // Add 16 empty fields for stats
			}

			// Add Fight History data if available
			if i < len(fighter.FightHistory) {
				history := fighter.FightHistory[i]
				record = append(record,
					history.Date, history.Opponent, history.Result,
					history.Decision, history.Round, history.Time, history.Event,
				)
			} else {
				record = append(record, make([]string, 7)...) // Add 7 empty fields for fight history
			}

			if err := writer.Write(record); err != nil {
				log.Fatal("Error writing record to CSV:", err)
			}
		}
	}

	fmt.Printf("Fighter data CSV file created: %s\n", csvFileName)
}

// Helper function to determine the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
