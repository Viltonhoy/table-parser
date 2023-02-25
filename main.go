package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gocolly/colly"
	"github.com/labstack/gommon/log"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type tableData struct {
	Code, Message string
}

type jsonStruct struct {
	Type         string `json:"type"`
	ProjectID    string `json:"project_id"`
	PrivatKeyID  string `json:"private_key_id"`
	PrivatKey    string `json:"private_key"`
	ClientEmail  string `json:"client_email"`
	ClientID     string `json:"client_id"`
	AuthURI      string `json:"auth_uri"`
	TokenURI     string `json:"token_uri"`
	AuthProvider string `json:"auth_provider_x509_cert_url"`
	Client       string `json:"client_x509_cert_url"`
}

type info struct {
	Data jsonStruct
}

func main() {
	var wg sync.WaitGroup
	var i info

	wg.Add(1)

	go func() {
		defer wg.Done()
		t := time.NewTicker(5 * time.Second)

		for range t.C {
			td := myParser()
			saveData(i, td)
		}
	}()

	wg.Wait()
}

func myParser() []tableData {
	var employeeData []tableData

	c := colly.NewCollector()

	c.WithTransport(&http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   90 * time.Second,
			KeepAlive: 60 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Scraping:", r.URL)
	})

	c.OnResponse(func(r *colly.Response) {
		fmt.Println("Status:", r.StatusCode)
	})

	c.OnHTML("table", func(h *colly.HTMLElement) {
		h.ForEach("tr", func(_ int, el *colly.HTMLElement) {
			tableData := tableData{
				Code:    el.ChildText("td:nth-child(1)"),
				Message: el.ChildText("td:nth-child(2)"),
			}
			employeeData = append(employeeData, tableData)
			fmt.Println(tableData)
		})
	})

	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err)
	})

	c.Visit("https://confluence.hflabs.ru/pages/viewpage.action?pageId=1181220999")
	return employeeData
}

func saveData(i info, tt []tableData) {
	ctx := context.Background()

	jsonFile, err := os.Open("myproject.json")
	if err != nil {
		log.Error(err)
		return
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	config, err := google.JWTConfigFromJSON(byteValue, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Error(err)
		return
	}

	client := config.Client(ctx)

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Error(err)
		return
	}

	sheetId := 0
	spreadSheetId := "12H-u2U-BkFv57xYEijDjN8Pvk7X4aW3HBbJZjNB2JP0"

	res1, err := srv.Spreadsheets.Get(spreadSheetId).Fields("sheets(properties(sheetId,title))").Do()
	if err != nil {
		log.Error(err)
		return
	}

	sheetName := ""
	for _, v := range res1.Sheets {
		props := v.Properties

		if props.SheetId == int64(sheetId) {
			sheetName = props.Title
			break
		}
	}

	record := make([][]interface{}, 0)

	for _, t := range tt {
		record = append(record,
			[]interface{}{
				t.Code,
				t.Message,
			})
	}

	records := sheets.ValueRange{
		Values: record,
	}

	res2, err := srv.Spreadsheets.Values.Update(spreadSheetId, sheetName, &records).ValueInputOption("USER_ENTERED").Context(ctx).Do()
	if err != nil || res2.HTTPStatusCode != 200 {
		log.Error(err)
		return
	}
}
