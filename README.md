# MMA Data Scraper

This is a Go-based web scraper that collects MMA event and fight data from ESPN's website. It extracts information about upcoming and past events, including event details and fight matchups.

## Features

- Scrapes MMA event data from ESPN's website
- Extracts event names, dates, locations, and fight matchups
- Handles concurrent scraping for improved performance
- Outputs data to a JSON file

## Requirements

- Go 1.16 or higher
- `github.com/gocolly/colly/v2` package

## Installation

1. Clone this repository:
   ```
   git clone https://github.com/yourusername/mma-data-scraper.git
   ```

2. Navigate to the project directory:
   ```
   cd mma-data-scraper
   ```

3. Install the required dependencies:
   ```
   go mod tidy
   ```

## Usage

To run the scraper, use the following command in the project directory:
```
go run main.go
```
