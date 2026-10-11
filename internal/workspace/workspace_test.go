package workspace

import "testing"

func TestValidate(t *testing.T) {
	values, err := Validate([]Domain{{Domain: "Example.COM", Completed: []string{"bimi", "spf", "spf"}, Selector: "selector1.mail"}})
	if err != nil || values[0].Domain != "example.com" || len(values[0].Completed) != 2 || values[0].Completed[0] != "spf" {
		t.Fatalf("%+v %v", values, err)
	}
	for _, values := range [][]Domain{{{Domain: "person@example.com"}}, {{Domain: "example.com", Completed: []string{"paid"}}}, {{Domain: "example.com", Selector: "../../secret"}}, {{Domain: "example.com"}, {Domain: "EXAMPLE.COM"}}, make([]Domain, 21)} {
		if _, err := Validate(values); err == nil {
			t.Fatalf("accepted %+v", values)
		}
	}
}
