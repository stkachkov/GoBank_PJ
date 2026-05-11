package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
)

var (
	ErrCentralBankServiceUnavailable = errors.New("central bank service is currently unavailable")
	ErrFailedToParseXMLResponse      = errors.New("failed to parse central bank XML response")
	ErrKeyRateNotFound               = errors.New("key rate not found in response")
	ErrInvalidKeyRateFormat          = errors.New("invalid key rate format in response")
)

const (
	cbrServiceURL = "https://www.cbr.ru/DailyInfoWebServ/DailyInfo.asmx"
	bankMargin    = 0
)

type CentralBankService struct {
	client *http.Client
}

func NewCentralBankService() *CentralBankService {
	return &CentralBankService{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (s *CentralBankService) GetKeyRate(ctx context.Context) (float64, error) {
	soapRequest := s.buildSOAPRequest()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", cbrServiceURL, bytes.NewBufferString(soapRequest))
	if err != nil {
		return 0, fmt.Errorf("failed to create SOAP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	httpReq.Header.Set("SOAPAction", "http://web.cbr.ru/KeyRate")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrCentralBankServiceUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("%w: received non-OK status %d: %s", ErrCentralBankServiceUnavailable, resp.StatusCode, string(bodyBytes))
	}

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("%w: failed to read SOAP response body: %v", ErrFailedToParseXMLResponse, err)
	}

	rate, err := s.parseXMLResponse(rawBody)
	if err != nil {
		return 0, err
	}

	rateWithMargin := rate + bankMargin
	return rateWithMargin, nil
}

func (s *CentralBankService) buildSOAPRequest() string {

	toDate := time.Now().Format("2006-01-02")

	fromDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<soap12:Envelope xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">
    <soap12:Body>
        <KeyRate xmlns="http://web.cbr.ru/">
            <fromDate>%s</fromDate>
            <ToDate>%s</ToDate>
        </KeyRate>
    </soap12:Body>
</soap12:Envelope>`, fromDate, toDate)
}

func (s *CentralBankService) parseXMLResponse(rawBody []byte) (float64, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawBody); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrFailedToParseXMLResponse, err)
	}

	rateElement := doc.FindElement("//KeyRate/KR/Rate")
	if rateElement == nil {
		rateElement = doc.FindElement("//*[local-name()='diffgram']//Rate")
		if rateElement == nil {
			return 0, ErrKeyRateNotFound
		}
	}

	rateStr := rateElement.Text()
	rate, err := strconv.ParseFloat(strings.Replace(rateStr, ",", ".", -1), 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidKeyRateFormat, err)
	}
	return rate, nil
}
