package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

// TOKEN - api key for the hetzner dns api
var TOKEN string = ""

// ZoneResponse - struct for all Zones
type ZoneResponse struct {
	Zones []struct {
		ID            string   `json:"id"`
		Name          string   `json:"name"`
		TTL           int      `json:"ttl"`
		Registrar     string   `json:"registrar"`
		LegacyDNSHost string   `json:"legacy_dns_host"`
		LegacyNs      []string `json:"legacy_ns"`
		Ns            []string `json:"ns"`
		Created       string   `json:"created"`
		Verified      string   `json:"verified"`
		Modified      string   `json:"modified"`
		Project       string   `json:"project"`
		Owner         string   `json:"owner"`
		Permission    string   `json:"permission"`
		ZoneType      struct {
			ID          string      `json:"id"`
			Name        string      `json:"name"`
			Description string      `json:"description"`
			Prices      interface{} `json:"prices"`
		} `json:"zone_type"`
		Status          string `json:"status"`
		Paused          bool   `json:"paused"`
		IsSecondaryDNS  bool   `json:"is_secondary_dns"`
		TxtVerification struct {
			Name  string `json:"name"`
			Token string `json:"token"`
		} `json:"txt_verification"`
		RecordsCount int `json:"records_count"`
	} `json:"zones"`
	Meta struct {
		Pagination struct {
			Page         int `json:"page"`
			PerPage      int `json:"per_page"`
			PreviousPage int `json:"previous_page"`
			NextPage     int `json:"next_page"`
			LastPage     int `json:"last_page"`
			TotalEntries int `json:"total_entries"`
		} `json:"pagination"`
	} `json:"meta"`
}

// DNSClient - simple client for hetzner dns
type DNSClient struct {
	Token string
}

func (c DNSClient) fetch(method string, function string, body []byte) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, fmt.Sprintf("https://dns.hetzner.com/api/v1/%s", function), bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Add("Auth-API-Token", c.Token)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	respBody, _ := ioutil.ReadAll(resp.Body)
	return string(respBody), nil
}

func getZoneID(zonename string) (string, error) {
	var c DNSClient
	c.Token = TOKEN
	zrd, _ := c.fetch("GET", "zones", nil)
	var zr ZoneResponse
	json.Unmarshal([]byte(zrd), &zr)
	for _, zone := range zr.Zones {
		if zone.Name == zonename {
			return zone.ID, nil
		}
	}
	return "", errors.New("no zone found")
}

// Record - a single dns record
type Record struct {
	ZoneID     string `json:"zone_id"`
	RecordType string `json:"type"`
	Name       string `json:"name"`
	Value      string `json:"value"`
	TTL        int    `json:"ttl"`
}

// RecordResponse - response of the hetzner api for all records
type RecordResponse struct {
	Records []struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Name     string `json:"name"`
		Value    string `json:"value"`
		ZoneID   string `json:"zone_id"`
		Created  string `json:"created"`
		Modified string `json:"modified"`
		TTL      int    `json:"ttl,omitempty"`
	} `json:"records"`
}

func (r Record) setRecord() {
	c := DNSClient{Token: TOKEN}
	json, _ := json.Marshal(r)
	ok, recordID := r.checkExisting()
	if ok {
		c.fetch("PUT", fmt.Sprintf("records/%s", recordID), json)
	} else {
		c.fetch("POST", "records", json)
	}
}

func (r Record) unsetRecord() {
	c := DNSClient{Token: TOKEN}
	ok, recordID := r.checkExisting()
	if ok {
		c.fetch("DELETE", fmt.Sprintf("records/%s", recordID), nil)
	} else {
		log.Fatalln("record does not exist")
	}
}

func (r Record) checkExisting() (bool, string) {
	c := DNSClient{Token: TOKEN}
	data, _ := c.fetch("GET", fmt.Sprintf("records?zone_id=%s", r.ZoneID), nil)
	var rr RecordResponse
	json.Unmarshal([]byte(data), &rr)
	for _, record := range rr.Records {
		if record.Name == r.Name && record.Type == r.RecordType {
			return true, record.ID
		}
	}
	return false, ""
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func main() {
	actionPtr := flag.Bool("delete", false, "delete the record")
	domainPtr := flag.String("domain", "test.example.com", "the domain to set/unset")
	typePtr := flag.String("type", "A", "the type of record to set")
	valuePtr := flag.String("value", "10.0.0.1", "the value to set for the record")
	tokenPtr := flag.String("token", "", "token for the dns api")
	tokenFile := flag.String("tokenFile", "/tmp/dnstoken.txt", "the text file including the token")

	flag.Parse()

	if fileExists(*tokenFile) {
		content, err := ioutil.ReadFile(*tokenFile)
		if err != nil {
			log.Fatal(err)
		}
		TOEKN := string(content)
	} else if len(*tokenPtr) < 1 {
		TOKEN = os.Getenv("DNS_API_TOKEN")
	} else {
		TOKEN = *tokenPtr
	}

	if len(TOKEN) < 5 {
		log.Fatalln("token seems invalid")
		os.Exit(1)
	}

	arr := strings.Split(*domainPtr, ".")
	zone := fmt.Sprintf("%s.%s", arr[len(arr)-2:][0], arr[len(arr)-1:][0])
	name := strings.Join(arr[:len(arr)-1][:len(arr)-2], ".")

	zoneID, err := getZoneID(zone)
	if err != nil {
		log.Fatalln(err)
	}

	r := Record{Name: name, RecordType: *typePtr, TTL: 10, Value: *valuePtr, ZoneID: zoneID}

	if *actionPtr {
		fmt.Println("deleting record")
		r.unsetRecord()
	} else {
		fmt.Println("installing record")
		r.setRecord()
	}
}
