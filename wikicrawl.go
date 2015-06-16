package wikicrawl

import (
  "fmt"
	"strings"
	"regexp"
	"encoding/json"
	"golang.org/x/net/html"
	"github.com/franela/goreq"
	"github.com/PuerkitoBio/goquery"
)

const ApiRoot string = "https://en.wikipedia.org/w/api.php"
const WikiContentRoot string = "https://en.wikipedia.org/wiki/"

type WikiParams struct {
	Format string
	Action string
	Page string
	Prop string
}

func nextPage(currentPage *WikiPage, pageChannel chan *WikiPage) {
	for {	
		// Go to the next page
		params := WikiParams{
			Action: "parse",
			Format: "json",
			Page: currentPage.Title(),
			Prop: "text",
		}
		
		req := goreq.Request{
			Uri: "https://en.wikipedia.org/w/api.php",
			QueryString: params,
		}
		
		// get the next page
		res, err := req.Do()
		
		if err == nil {
			if rawJson, err2 := res.Body.ToString(); err2 == nil {
				// parse the next page's json
				var jsonMap map[string]interface{}
				json.Unmarshal([]byte(rawJson), &jsonMap)
				// extract html
				var pageHtml string = jsonMap["parse"].(map[string]interface{})["text"].(map[string]interface{})["*"].(string)
				if page, err3 := html.Parse(strings.NewReader(pageHtml)); err3 == nil {
					// tokenize html into goquery
					document := goquery.NewDocumentFromNode(page)
					// check for a redirect
					redirectDivSelection := document.Find("div.redirectMsg")
					if redirectDivSelection.Length() > 0 {
						// we have a redirect!
						redirectAnchor := redirectDivSelection.Find("a").Eq(0)
						redirectTitle, _ := redirectAnchor.Attr("title")
						pageChannel <- NewWikiPage(redirectTitle)
						return
					}
					anchors := document.Find("p").Find("a")
					// search for anchors between <p> tags
					for i := range anchors.Nodes {
						if href, exists := anchors.Eq(i).Attr("href"); exists {
							contentLinkRegex := regexp.MustCompile("^/wiki/([^:]+$)")
							if submatches := contentLinkRegex.FindStringSubmatch(href); submatches != nil {
								// link points to a valid next page
								extractedTitle := submatches[1]
								pageChannel <- NewWikiPage(extractedTitle)
								return
							}
						}
					}
				} else {
					panic(err3)
				}
			} else {
				panic(err2)
			}
		} else {
			panic(err)
		}
	}
}

func randWikiUrl(output chan *WikiPage) {
	// Keep retrying until find non-disambiguation page
	for {
		redirectResponse, _ := goreq.Request{
			Uri: WikiContentRoot + "Special:Random",
		}.Do()
		// Obtain the random url from the redirect's location header
		redirectUrl, _ := redirectResponse.Location()
		redirectUrlString := redirectUrl.String()
		blacklistedRegex := regexp.MustCompile("(?i:(?:disambiguation)|(?:list_of|list of))")
		if blacklistedRegex.Find([]byte(redirectUrlString)) == nil {
			// not a blacklisted page
			randomTitle := strings.Replace(redirectUrlString, WikiContentRoot, "", 1)
			output <- NewWikiPage(randomTitle)
		}
	}
}

func Crawl(startPage *WikiPage, resultsChannel chan CrawlResult) {
	const MAX_JUMPS int = 100
	results := make([]WikiPage, 0, MAX_JUMPS + 1)
	visited := make(map[string]bool)

	var currentPage *WikiPage = startPage
	nextPageChan := make(chan *WikiPage, 1)

	for i := 0; i < MAX_JUMPS; i++ {
		// record visitation of the page
		results = append(results, *currentPage)
		visited[currentPage.Title()] = true
		fmt.Println("Visiting", currentPage.Title())

		// Go to the next page
		go nextPage(currentPage, nextPageChan)
		currentPage = <- nextPageChan
		a, b := visited[currentPage.Title()]
		fmt.Printf("Value: %s, hasKey %s\n", a, b)
		if _, hasKey := visited[currentPage.Title()]; hasKey {
			// already been here
			fmt.Println("already been here")
			break
		}
	}
	resultsChannel <- CrawlResult{results}
}

// Dummy function that allows main() to be run from other programs
func Run() {
	main()
}

func main() {

	wikipageChannel := make(chan *WikiPage, 1)
	go randWikiUrl(wikipageChannel)
	startPage := <- wikipageChannel
	fmt.Println(startPage)
	fmt.Println(startPage.Title())

	resultsChan := make(chan CrawlResult)
	go Crawl(startPage, resultsChan)
	fmt.Println("Done!", <- resultsChan)
}
