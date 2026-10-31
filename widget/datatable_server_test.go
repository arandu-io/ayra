package widget_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"testing"

	"github.com/arandu-io/ayra/client"
	"github.com/arandu-io/ayra/widget"
)

// answer is what a route answers an app that asks for the values: the envelope
// of hesape's ViewData, around the struct the handler gave its view. The pager's
// numbers are those of hesape's LengthAwarePage.MarshalJSON.
const answer = `{
  "view": "cameras::index",
  "token": "t",
  "data": {
    "Page": {"Title": "Câmeras"},
    "Table": {
      "URL": "/painel/cameras",
      "Result": {
        "State": {},
        "Page": {"current_page": 2, "per_page": 2, "last_page": 3, "total": 5, "from": 3, "to": 4},
        "facets": {"status": [{"value": "online", "count": 3}, {"value": "parada", "count": 2}]}
      }
    },
    "Rows": [
      {"ID": "c3", "Name": "Garagem", "StatusLabel": "Online", "Channels": 4, "Active": true},
      {"ID": "c4", "Name": "Portão", "StatusLabel": "Parada", "Channels": 1, "Active": false}
    ]
  }
}`

type getter struct {
	got    []client.Path
	answer string
	err    error
}

func (g *getter) Get(_ context.Context, path client.Path) (client.Page, error) {
	g.got = append(g.got, path)
	if g.err != nil {
		return client.Page{}, g.err
	}
	var page client.Page
	err := json.Unmarshal([]byte(g.answer), &page)
	return page, err
}

func cameras(g *getter) widget.ServerSource {
	return widget.ServerSource{
		Client: g, Path: client.At("painel", "cameras"), DefaultSize: 20,
		Fields: []string{"Name", "StatusLabel", "Channels"},
	}
}

func TestAServerSourceAsksTheRoutesAddressWithTheTablesState(t *testing.T) {
	g := &getter{answer: answer}
	_, err := cameras(g).Fetch(context.Background(), widget.Query{
		Search: "gar", Sort: widget.Sort{Key: "name"}, Page: 2, PageSize: 2,
		Filters: map[string][]string{"status": {"online", "parada"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.got) != 1 {
		t.Fatalf("the route was asked %d times", len(g.got))
	}
	u, err := url.Parse(g.got[0].String())
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/painel/cameras" {
		t.Errorf("the route asked is %s", u.Path)
	}
	want := url.Values{
		"q": {"gar"}, "sort": {"name"}, "dir": {"asc"}, "page": {"2"}, "per_page": {"2"},
		"status": {"online", "parada"},
	}
	if got := u.Query(); !reflect.DeepEqual(got, want) {
		t.Errorf("the query is %v, want %v", got, want)
	}
}

func TestAServerSourceReadsRowsTotalAndCountsFromTheEnvelope(t *testing.T) {
	g := &getter{answer: answer}
	result, err := cameras(g).Fetch(context.Background(), widget.Query{Page: 2, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 5 {
		t.Errorf("total is %d, want the 5 the listing's page says", result.Total)
	}
	if len(result.Rows) != 2 || result.Rows[0].Key != "c3" {
		t.Fatalf("rows are %+v", result.Rows)
	}
	if want := []string{"Garagem", "Online", "4"}; !reflect.DeepEqual(result.Rows[0].Cells, want) {
		t.Errorf("cells are %v, want %v", result.Rows[0].Cells, want)
	}
	if result.Rows[1].Data["Active"] != false {
		t.Errorf("the row's data was not kept: %v", result.Rows[1].Data)
	}
	if got := result.Facets["status"]; len(got) != 2 || got[0] != (widget.FacetCount{Value: "online", Count: 3}) {
		t.Errorf("facet counts are %v", result.Facets)
	}
}

func TestAServerSourceFindsTheNamesWhateverTheirCase(t *testing.T) {
	g := &getter{answer: `{"view":"x","data":{"rows":[{"id":7,"name":"a"}],"total":1}}`}
	source := widget.ServerSource{Client: g, Fields: []string{"Name"}}
	result, err := source.Fetch(context.Background(), widget.Query{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || result.Rows[0].Key != "7" || result.Rows[0].Cells[0] != "a" {
		t.Errorf("the tagged struct was read as %+v", result)
	}
}

func TestARouteThatDoesNotCountLeavesTheTotalUnknown(t *testing.T) {
	g := &getter{answer: `{"view":"x","data":{"Rows":[{"ID":"1"}]}}`}
	result, err := widget.ServerSource{Client: g}.Fetch(context.Background(), widget.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total >= 0 {
		t.Errorf("total is %d for a route that says none", result.Total)
	}
}

func TestAnEmptyListIsNotAnError(t *testing.T) {
	for _, body := range []string{`{"data":{"Rows":null}}`, `{"data":{"Rows":[]}}`} {
		g := &getter{answer: body}
		result, err := widget.ServerSource{Client: g}.Fetch(context.Background(), widget.Query{})
		if err != nil || len(result.Rows) != 0 {
			t.Errorf("%s: rows=%v err=%v", body, result.Rows, err)
		}
	}
}

func TestARouteThatIsNotAListIsSaidSo(t *testing.T) {
	g := &getter{answer: `{"view":"auth::login","data":{"Title":"Entrar"}}`}
	_, err := widget.ServerSource{Client: g}.Fetch(context.Background(), widget.Query{})
	if err == nil {
		t.Fatal("a view with no list of rows was taken for an empty list")
	}
}

func TestTheServersRefusalReachesTheTable(t *testing.T) {
	refused := &client.StatusError{}
	g := &getter{err: refused}
	_, err := cameras(g).Fetch(context.Background(), widget.Query{})
	if !errors.Is(err, refused) {
		t.Errorf("the error is %v: a 404 is the server's answer and the table has to be able to say it", err)
	}
	if _, err := (widget.ServerSource{}).Fetch(context.Background(), widget.Query{}); err == nil {
		t.Error("a source with no client asked nothing and said nothing")
	}
}

func TestTheFieldsOfTheColumnsAreTheirKeys(t *testing.T) {
	cols := []widget.DataColumn{{Key: "name"}, {Key: "status"}}
	if got := widget.FieldsOf(cols); !reflect.DeepEqual(got, []string{"name", "status"}) {
		t.Errorf("fields are %v", got)
	}
}
