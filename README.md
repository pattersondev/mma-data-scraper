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

The scraper will start collecting data from ESPN's MMA pages. It will print progress information to the console as it runs.

## Output

After the scraper finishes, it will create a JSON file in the same directory with a name in the format:
```
events[YYYY-MM-DD_HH-MM-SS].json
```

his file contains an array of Event objects, each including:
- Event name
- Date
- Location
- List of fight matchups (fighter names and results, if available)

## Customization

You can adjust the number of concurrent workers by modifying the `for i := 0; i < 5; i++` loop in the `scrapeData` function. Increase or decrease the number based on your system's capabilities and the desired scraping speed.

## Notes

- This scraper is designed for educational and personal use. Make sure to comply with ESPN's terms of service and robots.txt file when using this tool.
- The scraper may need updates if ESPN changes their website structure.
- Some commented-out code for fighter-specific data (stats, bio, fight history) is included but not currently used. You can uncomment and modify these sections if you want to collect more detailed fighter information.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is open source and available under the [MIT License](LICENSE).
