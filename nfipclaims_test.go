package nfipclaims

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

const femaNotice = "This product uses the Federal Emergency Management Agency’s OpenFEMA API, but is not endorsed by FEMA. The Federal Government or FEMA cannot vouch for the data or analyses derived from these data after the data have been retrieved from the Agency's website(s)."

type jsonRecord struct {
	State              string   `json:"state"`
	Window             string   `json:"window"`
	AsOfDate           string   `json:"as_of_date"`
	RetrievedUTC       string   `json:"retrieved_utc"`
	NClaims            int      `json:"n_claims"`
	NPaid              int      `json:"n_paid"`
	Q1                 *float64 `json:"q1"`
	Median             *float64 `json:"median"`
	Q3                 *float64 `json:"q3"`
	TotalPaid          *float64 `json:"total_paid"`
	Median2024USD      *float64 `json:"median_2024usd"`
	TopEvent           *string  `json:"top_event"`
	TopEventShare      *float64 `json:"top_event_share"`
	MedianExclTopEvent *float64 `json:"median_excl_top_event"`
	EventDependent     *bool    `json:"event_dependent"`
	Status             string   `json:"status"`
}

type jsonDoc struct {
	Source struct {
		Endpoint     string `json:"endpoint"`
		Version      string `json:"version"`
		RetrievedUTC string `json:"retrieved_utc"`
		AsOfDate     string `json:"as_of_date"`
	} `json:"source"`
	Method struct {
		Window string `json:"window"`
	} `json:"method"`
	Records []jsonRecord `json:"records"`
}

func loadDoc(t *testing.T) jsonDoc {
	t.Helper()
	raw, err := os.ReadFile("testdata/nfip_paid_claims_by_state.json")
	if err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256(raw); hex.EncodeToString(sum[:]) != sourceSHA256 {
		t.Fatal("testdata differs from the file nfip_gen.go was generated from: run tools/gen_go.py")
	}
	var d jsonDoc
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func f(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func TestEveryFieldMatchesJSON(t *testing.T) {
	d := loadDoc(t)
	got := States()
	if len(d.Records) != 51 || len(got) != 51 {
		t.Fatalf("records: json %d, go %d, want 51", len(d.Records), len(got))
	}
	if AsOfDate != d.Source.AsOfDate || RetrievedUTC != d.Source.RetrievedUTC || Endpoint != d.Source.Endpoint ||
		Window != d.Method.Window || d.Source.Version != "v3" {
		t.Fatalf("constants differ from source metadata: %+v", d.Source)
	}
	for i, r := range d.Records {
		g := got[i]
		insufficient := r.Status == StatusInsufficient
		for name, p := range map[string]*float64{"q1": r.Q1, "median": r.Median, "q3": r.Q3, "total_paid": r.TotalPaid,
			"median_2024usd": r.Median2024USD, "top_event_share": r.TopEventShare} {
			if (p == nil) != insufficient {
				t.Errorf("%s: %s null=%v with status %q", r.State, name, p == nil, r.Status)
			}
		}
		topEvent := ""
		if r.TopEvent != nil {
			topEvent = *r.TopEvent
		}
		want := StateStats{
			State: r.State, Window: r.Window, AsOfDate: r.AsOfDate, RetrievedUTC: r.RetrievedUTC,
			NClaims: r.NClaims, NPaid: r.NPaid, Q1: f(r.Q1), Median: f(r.Median), Q3: f(r.Q3),
			TotalPaid: f(r.TotalPaid), Median2024USD: f(r.Median2024USD), TopEvent: topEvent,
			TopEventShare: f(r.TopEventShare), MedianExclTopEvent: f(r.MedianExclTopEvent),
			HasMedianExclTopEvent: r.MedianExclTopEvent != nil,
			EventDependent:        r.EventDependent != nil && *r.EventDependent, Status: r.Status,
		}
		if (r.EventDependent == nil) != (r.MedianExclTopEvent == nil) {
			t.Errorf("%s: event_dependent and median_excl_top_event disagree on null", r.State)
		}
		if g != want {
			t.Errorf("%s:\n go   %+v\n json %+v", r.State, g, want)
		}
	}
}

func TestLookupAndPosition(t *testing.T) {
	fl, ok := Lookup(" fl ")
	if !ok || fl.State != "FL" || fl.Status != StatusOK {
		t.Fatalf("Lookup FL: %+v %v", fl, ok)
	}
	if _, ok := Lookup("ZZ"); ok {
		t.Error("Lookup ZZ: ok")
	}
	cases := []struct {
		amount float64
		want   int
	}{{0, 1}, {fl.Q1, 1}, {fl.Q1 + 0.01, 2}, {fl.Median, 2}, {fl.Q3, 3}, {fl.Q3 + 0.01, 4}}
	for _, c := range cases {
		if q, ok := Position("FL", c.amount); !ok || q != c.want {
			t.Errorf("Position(FL, %v) = %d, %v; want %d", c.amount, q, ok, c.want)
		}
	}
	for _, s := range States() {
		if _, ok := Position(s.State, s.Median); ok != (s.Status == StatusOK) {
			t.Errorf("Position(%s): ok=%v with status %q", s.State, ok, s.Status)
		}
	}
	if _, ok := Position("ZZ", 1); ok {
		t.Error("Position ZZ: ok")
	}
}

func TestStatesIsACopy(t *testing.T) {
	s := States()
	s[0].State = "XX"
	if States()[0].State == "XX" {
		t.Error("States returns the package slice")
	}
}

func TestDocComment(t *testing.T) {
	src, err := os.ReadFile("nfipclaims.go")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(src[:strings.Index(string(src), "\npackage nfipclaims")])
	for _, s := range []string{femaNotice, "amounts paid by NFIP, not repair cost", "version v3", "asOfDate of the records is " + AsOfDate,
		"\n// " + Endpoint + "\n", "\n// https://restoreworth.com/\n", "\n// https://restoreworth.com/nfip-flood-claims-by-state.html\n",
		"not an official federal report"} {
		if !strings.Contains(doc, s) {
			t.Errorf("package doc lacks %q", s)
		}
	}
	for _, name := range []string{"README.md", "DATA_NOTICE.md"} {
		if b, err := os.ReadFile(name); err != nil || !strings.Contains(string(b), "> "+femaNotice+"\n") {
			t.Errorf("%s lacks the FEMA notice (%v)", name, err)
		}
	}
}

// TestEveryFieldMatchesCSV compares every value with the published CSV, cell by
// cell, and the whole-dollar rounding (half up, as the site page) of the medians.
func TestEveryFieldMatchesCSV(t *testing.T) {
	fh, err := os.Open("testdata/nfip_paid_claims_by_state.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	rows, err := csv.NewReader(fh).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	got := States()
	if len(rows) != 52 || len(got) != 51 {
		t.Fatalf("csv rows %d, go %d; want 52 and 51", len(rows), len(got))
	}
	num := func(cell string, v float64) bool {
		if cell == "" {
			return v == 0
		}
		x, err := strconv.ParseFloat(cell, 64)
		return err == nil && x == v
	}
	for i, r := range rows[1:] {
		g := got[i]
		ed := ""
		if g.HasMedianExclTopEvent {
			ed = strconv.FormatBool(g.EventDependent)
		}
		strs := []string{g.State, g.Window, g.AsOfDate, g.RetrievedUTC, strconv.Itoa(g.NClaims), strconv.Itoa(g.NPaid)}
		ok := strings.Join(r[:6], ",") == strings.Join(strs, ",") && r[11] == g.TopEvent && r[14] == ed && r[15] == g.Status &&
			num(r[6], g.Q1) && num(r[7], g.Median) && num(r[8], g.Q3) && num(r[9], g.TotalPaid) && num(r[10], g.Median2024USD) &&
			num(r[12], g.TopEventShare) && num(r[13], g.MedianExclTopEvent) && (r[13] == "") != g.HasMedianExclTopEvent
		if !ok {
			t.Errorf("%s: go %+v\n csv %v", r[0], g, r)
		}
	}
	if ky, _ := Lookup("KY"); math.Round(ky.Median) != 17533 {
		t.Errorf("KY median rounds to %v, the page shows 17,533", math.Round(ky.Median))
	}
}
