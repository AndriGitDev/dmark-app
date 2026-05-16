package ingest

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
)

const DefaultMaxReportBytes int64 = 10 << 20

var (
	ErrUnsupportedReport = errors.New("unsupported DMARC report payload")
	ErrUnsafeZipPath     = errors.New("unsafe zip entry path")
	ErrReportTooLarge    = errors.New("DMARC report payload is too large")
)

type AggregateReport struct {
	ReportID    string
	OrgName     string
	Email       string
	Domain      string
	BeginUnix   int64
	EndUnix     int64
	ContentHash string
	SourceFile  string
	Policy      PolicyPublished
	Records     []AggregateRecord
	AuthResults []AuthResult
	RawXML      []byte
}

type PolicyPublished struct {
	ADKIM string
	ASPF  string
	P     string
	SP    string
	Pct   int
}

type AggregateRecord struct {
	SourceIP          net.IP
	Count             int
	Disposition       string
	DKIMAlignedResult string
	SPFAlignedResult  string
	HeaderFrom        string
}

type AuthResult struct {
	Kind     string
	Domain   string
	Selector string
	Result   string
}

func ParseAggregatePayload(ctx context.Context, filename string, payload []byte, maxBytes int64) ([]AggregateReport, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxReportBytes
	}
	xmlFiles, err := extractXMLFiles(filename, payload, maxBytes)
	if err != nil {
		return nil, err
	}
	reports := make([]AggregateReport, 0, len(xmlFiles))
	for _, file := range xmlFiles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		report, err := parseAggregateXML(file.name, file.data)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

type xmlFile struct {
	name string
	data []byte
}

func extractXMLFiles(filename string, payload []byte, maxBytes int64) ([]xmlFile, error) {
	if int64(len(payload)) > maxBytes {
		return nil, ErrReportTooLarge
	}
	lowerName := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lowerName, ".gz"):
		data, err := gunzip(payload, maxBytes)
		if err != nil {
			return nil, err
		}
		return []xmlFile{{name: strings.TrimSuffix(filename, filepath.Ext(filename)), data: data}}, nil
	case strings.HasSuffix(lowerName, ".zip"):
		return unzipXML(filename, payload, maxBytes)
	default:
		if bytes.HasPrefix(bytes.TrimSpace(payload), []byte("<")) {
			return []xmlFile{{name: filename, data: payload}}, nil
		}
		return nil, ErrUnsupportedReport
	}
}

func gunzip(payload []byte, maxBytes int64) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	return readLimited(reader, maxBytes)
}

func unzipXML(filename string, payload []byte, maxBytes int64) ([]xmlFile, error) {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, err
	}
	files := []xmlFile{}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if filepath.IsAbs(file.Name) || strings.Contains(file.Name, "..") {
			return nil, fmt.Errorf("%w: %s", ErrUnsafeZipPath, file.Name)
		}
		if !strings.HasSuffix(strings.ToLower(file.Name), ".xml") {
			continue
		}
		opened, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := readLimited(opened, maxBytes)
		closeErr := opened.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		files = append(files, xmlFile{name: file.Name, data: data})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: %s contains no XML reports", ErrUnsupportedReport, filename)
	}
	return files, nil
}

func readLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(reader, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrReportTooLarge
	}
	return data, nil
}

func parseAggregateXML(filename string, data []byte) (AggregateReport, error) {
	var raw feedbackXML
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&raw); err != nil {
		return AggregateReport{}, fmt.Errorf("parsing DMARC report %s: %w", filename, err)
	}
	hash := sha256.Sum256(data)
	report := AggregateReport{
		ReportID:    strings.TrimSpace(raw.ReportMetadata.ReportID),
		OrgName:     strings.TrimSpace(raw.ReportMetadata.OrgName),
		Email:       strings.TrimSpace(raw.ReportMetadata.Email),
		Domain:      strings.TrimSpace(raw.PolicyPublished.Domain),
		BeginUnix:   raw.ReportMetadata.DateRange.Begin,
		EndUnix:     raw.ReportMetadata.DateRange.End,
		ContentHash: hex.EncodeToString(hash[:]),
		SourceFile:  filename,
		Policy: PolicyPublished{
			ADKIM: strings.TrimSpace(raw.PolicyPublished.ADKIM),
			ASPF:  strings.TrimSpace(raw.PolicyPublished.ASPF),
			P:     strings.TrimSpace(raw.PolicyPublished.P),
			SP:    strings.TrimSpace(raw.PolicyPublished.SP),
			Pct:   raw.PolicyPublished.Pct,
		},
		RawXML: append([]byte(nil), data...),
	}
	for _, rawRecord := range raw.Records {
		record := AggregateRecord{
			SourceIP:          net.ParseIP(strings.TrimSpace(rawRecord.Row.SourceIP)),
			Count:             rawRecord.Row.Count,
			Disposition:       strings.TrimSpace(rawRecord.Row.PolicyEvaluated.Disposition),
			DKIMAlignedResult: strings.TrimSpace(rawRecord.Row.PolicyEvaluated.DKIM),
			SPFAlignedResult:  strings.TrimSpace(rawRecord.Row.PolicyEvaluated.SPF),
			HeaderFrom:        strings.TrimSpace(rawRecord.Identifiers.HeaderFrom),
		}
		if record.SourceIP == nil {
			return AggregateReport{}, fmt.Errorf("parsing DMARC report %s: invalid source IP %q", filename, rawRecord.Row.SourceIP)
		}
		report.Records = append(report.Records, record)
		for _, dkim := range rawRecord.AuthResults.DKIM {
			report.AuthResults = append(report.AuthResults, AuthResult{Kind: "dkim", Domain: strings.TrimSpace(dkim.Domain), Selector: strings.TrimSpace(dkim.Selector), Result: strings.TrimSpace(dkim.Result)})
		}
		for _, spf := range rawRecord.AuthResults.SPF {
			report.AuthResults = append(report.AuthResults, AuthResult{Kind: "spf", Domain: strings.TrimSpace(spf.Domain), Result: strings.TrimSpace(spf.Result)})
		}
	}
	if report.ReportID == "" || report.Domain == "" || len(report.Records) == 0 {
		return AggregateReport{}, fmt.Errorf("parsing DMARC report %s: missing required metadata or records", filename)
	}
	return report, nil
}

type feedbackXML struct {
	XMLName         xml.Name             `xml:"feedback"`
	ReportMetadata  reportMetadataXML    `xml:"report_metadata"`
	PolicyPublished policyPublishedXML   `xml:"policy_published"`
	Records         []aggregateRecordXML `xml:"record"`
}

type reportMetadataXML struct {
	OrgName   string       `xml:"org_name"`
	Email     string       `xml:"email"`
	ReportID  string       `xml:"report_id"`
	DateRange dateRangeXML `xml:"date_range"`
}

type dateRangeXML struct {
	Begin int64 `xml:"begin"`
	End   int64 `xml:"end"`
}

type policyPublishedXML struct {
	Domain string `xml:"domain"`
	ADKIM  string `xml:"adkim"`
	ASPF   string `xml:"aspf"`
	P      string `xml:"p"`
	SP     string `xml:"sp"`
	Pct    int    `xml:"pct"`
}

type aggregateRecordXML struct {
	Row         rowXML         `xml:"row"`
	Identifiers identifiersXML `xml:"identifiers"`
	AuthResults authResultsXML `xml:"auth_results"`
}

type rowXML struct {
	SourceIP        string             `xml:"source_ip"`
	Count           int                `xml:"count"`
	PolicyEvaluated policyEvaluatedXML `xml:"policy_evaluated"`
}

type policyEvaluatedXML struct {
	Disposition string `xml:"disposition"`
	DKIM        string `xml:"dkim"`
	SPF         string `xml:"spf"`
}

type identifiersXML struct {
	HeaderFrom string `xml:"header_from"`
}

type authResultsXML struct {
	DKIM []dkimAuthXML `xml:"dkim"`
	SPF  []spfAuthXML  `xml:"spf"`
}

type dkimAuthXML struct {
	Domain   string `xml:"domain"`
	Selector string `xml:"selector"`
	Result   string `xml:"result"`
}

type spfAuthXML struct {
	Domain string `xml:"domain"`
	Result string `xml:"result"`
}
