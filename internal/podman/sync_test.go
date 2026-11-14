package podman

import (
	"net"
	"strings"
	"testing"
	"time"
)

func host(id, name, ref, addr string, port int) Host {
	h := Host{ID: id, Name: name, Ref: ref, Access: "home"}
	h.Target.Address, h.Target.Port = addr, port
	return h
}

var env = Env{Self: "reverse-proxy", Reach: Reach{Networks: []string{"podman"}}, HostAddress: "192.168.1.5"}

func TestReconcileCreateUpdateDelete(t *testing.T) {
	radarr := ct("radarr", map[string]string{LabelOn: "true", LabelImagePort: "7878"})
	sonarr := ct("sonarr", map[string]string{LabelName: "tv", LabelImagePort: "8989"})
	sonarr.IPs["podman"] = "10.88.0.6"
	self := ct("reverse-proxy", map[string]string{LabelOn: "true", LabelImagePort: "8443"})
	hosts := []Host{
		host("h1", "radarr", "podman:radarr", "10.88.0.50", 7878), // moved: new IP
		host("h2", "gone", "podman:gone", "10.88.0.7", 80),        // container removed
		host("h3", "paperless", "fjord:1:paperless", "10.88.0.8", 8000),
	}
	hosts[0].TrustedNetworks = []string{"10.0.0.0/8"}
	p := Reconcile([]Container{radarr, sonarr, self}, hosts, env)
	if len(p.Create) != 1 || p.Create[0].Name != "tv" || p.Create[0].Target.Address != "10.88.0.6" || p.Create[0].Ref != "podman:sonarr" {
		t.Fatalf("create: %+v", p.Create)
	}
	up, ok := p.Update["h1"]
	if !ok || up.Target.Address != "10.88.0.5" || strings.Join(up.TrustedNetworks, ",") != "10.0.0.0/8" {
		t.Fatalf("update keeps the UI's fields: %+v", p.Update)
	}
	if len(p.Delete) != 0 {
		t.Fatalf("a gone container's name is kept, not deleted: %v", p.Delete)
	}
	if g, ok := p.Update["h2"]; !ok || g.Gone == nil || !*g.Gone || g.Target.Address != "10.88.0.7" {
		t.Fatalf("a gone container's name is marked gone, target kept: %+v", p.Update["h2"])
	}
	if _, ok := p.Update["h3"]; ok {
		t.Fatal("a fjord name is not touched")
	}
	if len(p.Problems) != 0 {
		t.Fatalf("problems: %v", p.Problems)
	}
}

func TestReconcileUnchangedIsQuiet(t *testing.T) {
	radarr := ct("radarr", map[string]string{LabelOn: "true", LabelImagePort: "7878"})
	h := host("h1", "radarr", "podman:radarr", "10.88.0.5", 7878)
	p := Reconcile([]Container{radarr}, []Host{h}, env)
	if len(p.Create)+len(p.Update)+len(p.Delete)+len(p.Problems) != 0 {
		t.Fatalf("expected nothing: %+v", p)
	}
}

func TestReconcileNameTakenIsReportedNotStolen(t *testing.T) {
	a := ct("radarr", map[string]string{LabelName: "movies", LabelImagePort: "7878"})
	b := ct("radarr-4k", map[string]string{LabelName: "movies", LabelImagePort: "7878"})
	byHand := host("h9", "movies", "", "192.168.1.9", 7878)
	p := Reconcile([]Container{a, b}, []Host{byHand}, env)
	if len(p.Create) != 0 || len(p.Problems) != 2 {
		t.Fatalf("hand-made name kept: %+v", p)
	}
	for _, pr := range p.Problems {
		if !strings.Contains(pr.Message, "taken by you") {
			t.Fatalf("problem wording: %+v", pr)
		}
	}
	p = Reconcile([]Container{a, b}, nil, env)
	if len(p.Create) != 1 || p.Create[0].Ref != "podman:radarr" || len(p.Problems) != 1 || !strings.Contains(p.Problems[0].Message, "taken by container radarr") {
		t.Fatalf("first seen keeps it: %+v", p)
	}
	// a host that holds the name keeps it while the other is reported
	existing := host("h1", "movies", "podman:radarr-4k", "10.88.0.5", 7878)
	p = Reconcile([]Container{a, b}, []Host{existing}, env)
	if len(p.Delete) != 0 || len(p.Create) != 0 || len(p.Problems) != 1 || p.Problems[0].Container != "radarr" {
		t.Fatalf("existing holder wins: %+v", p)
	}
}

func TestReconcileProblemsKeepTheHost(t *testing.T) {
	c := ct("app", map[string]string{LabelOn: "true"}) // no port anywhere
	h := host("h1", "app", "podman:app", "10.88.0.5", 80)
	p := Reconcile([]Container{c}, []Host{h}, env)
	if len(p.Delete) != 0 || len(p.Problems) != 1 {
		t.Fatalf("a container with a problem is not deleted, it is reported: %+v", p)
	}
}

func TestReconcilePublicAndNote(t *testing.T) {
	c := ct("app", map[string]string{LabelOn: "true", LabelImagePort: "80", LabelPublic: "true"})
	p := Reconcile([]Container{c}, nil, env)
	if p.Create[0].Access != "internet" || *p.Create[0].Note != "" {
		t.Fatalf("public: %+v", p.Create[0])
	}
	e := env
	e.HomeOnly = true
	p = Reconcile([]Container{c}, nil, e)
	if p.Create[0].Access != "home" || !strings.Contains(*p.Create[0].Note, "home only") {
		t.Fatalf("home only: %+v", p.Create[0])
	}
}

func TestReconcileStoppedKeepsTheName(t *testing.T) {
	c := ct("radarr", map[string]string{LabelOn: "true", LabelImagePort: "7878"})
	c.State, c.IPs = "exited", map[string]string{}
	h := host("h1", "radarr", "podman:radarr", "10.88.0.5", 7878)
	p := Reconcile([]Container{c}, []Host{h}, env)
	if len(p.Delete)+len(p.Update)+len(p.Problems) != 0 {
		t.Fatalf("stopped is not gone and not a problem: %+v", p)
	}
	if p = Reconcile([]Container{c}, nil, env); len(p.Create)+len(p.Problems) != 0 {
		t.Fatalf("never started: nothing yet, no problem: %+v", p)
	}
}

func fjordWho(h Host) string {
	if strings.HasPrefix(h.Ref, "fjord:") {
		return "fjord on jupiter"
	}
	return ""
}

func TestReconcileSameNameSamePlaceIsNotAProblem(t *testing.T) {
	// jupiter: fjord names radarr at its epair address; the label asks the same
	radarr := ct("radarr", map[string]string{LabelOn: "true", LabelImagePort: "7878"})
	radarr.Networks, radarr.IPs, radarr.Addrs = []string{"vlan5"}, map[string]string{}, []net.IP{net.ParseIP("192.168.5.10")}
	fj := host("f1", "radarr", "fjord:e815:radarr", "192.168.5.10", 7878)
	e := env
	_, own, _ := net.ParseCIDR("192.168.5.145/24")
	e.Reach = Reach{Networks: []string{"vlan5"}, Subnets: []*net.IPNet{own}}
	e.Who = fjordWho
	p := Reconcile([]Container{radarr}, []Host{fj}, e)
	if len(p.Create)+len(p.Update)+len(p.Delete)+len(p.Problems) != 0 {
		t.Fatalf("one app, two sources: nothing to do, no problem: %+v", p)
	}
	if len(p.Elsewhere) != 1 || p.Elsewhere[0].Message != "already named radarr by fjord on jupiter" {
		t.Fatalf("elsewhere: %+v", p.Elsewhere)
	}
}

func TestReconcileSameNameOtherPlaceSaysWho(t *testing.T) {
	radarr := ct("radarr", map[string]string{LabelOn: "true", LabelImagePort: "7878"})
	fj := host("f1", "radarr", "fjord:e815:radarr", "192.168.5.99", 7878)
	e := env
	e.Who = fjordWho
	p := Reconcile([]Container{radarr}, []Host{fj}, e)
	if len(p.Create) != 0 || len(p.Problems) != 1 || p.Problems[0].Message != "name radarr is taken by fjord on jupiter" {
		t.Fatalf("clash: %+v", p)
	}
}

func TestReconcileOtherNameSamePlaceIsNoted(t *testing.T) {
	// published port, named by fjord through the host's name
	sonarr := ct("sonarr", map[string]string{LabelName: "tv", LabelImagePort: "8989"})
	sonarr.Networks, sonarr.IPs, sonarr.Addrs = []string{"other"}, map[string]string{"other": "10.89.0.4"}, nil
	sonarr.Published[8989] = 18989
	fj := host("f1", "sonarr", "fjord:e815:sonarr", "jupiter", 18989)
	e := env
	e.HostNames, e.Who = []string{"jupiter.ahze.lan", "192.168.1.5"}, fjordWho
	p := Reconcile([]Container{sonarr}, []Host{fj}, e)
	if len(p.Create) != 1 || p.Create[0].Name != "tv" || len(p.Problems) != 0 {
		t.Fatalf("a second name is allowed: %+v", p)
	}
	if len(p.Elsewhere) != 1 || p.Elsewhere[0].Message != "also named sonarr by fjord on jupiter" {
		t.Fatalf("elsewhere: %+v", p.Elsewhere)
	}
}

// fjord's Stop runs `compose down`, which deletes the container: its name
// stays, marked gone (uptime shows it down), comes back with the container,
// and goes only after GoneFor.
func TestGoneContainerKeepsItsNameForAWeek(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	e := env
	e.Now = now
	h := host("h1", "lidarr", "podman:lidarr_lidarr_1", "10.88.0.5", 8686)

	p := Reconcile(nil, []Host{h}, e)
	if g := p.Update["h1"]; g.Gone == nil || !*g.Gone || len(p.Delete) != 0 {
		t.Fatalf("just gone: marked, kept: %+v", p)
	}

	since := now.Add(-6 * 24 * time.Hour)
	h.GoneSince = &since
	p = Reconcile(nil, []Host{h}, e)
	if len(p.Update)+len(p.Delete) != 0 {
		t.Fatalf("gone 6 days: nothing to do: %+v", p)
	}

	since = now.Add(-8 * 24 * time.Hour)
	p = Reconcile(nil, []Host{h}, e)
	if len(p.Delete) != 1 || p.Delete[0] != "h1" {
		t.Fatalf("gone 8 days: removed: %+v", p)
	}

	// back, on a new address (fjord Start after Stop)
	since = now.Add(-time.Hour)
	c := ct("lidarr_lidarr_1", map[string]string{LabelName: "lidarr", LabelPort: "8686"})
	c.IPs["podman"], c.Addrs = "10.88.0.9", []net.IP{net.ParseIP("10.88.0.9")}
	p = Reconcile([]Container{c}, []Host{h}, e)
	up, ok := p.Update["h1"]
	if !ok || up.Gone == nil || *up.Gone || up.Target.Address != "10.88.0.9" || len(p.Create)+len(p.Delete) != 0 {
		t.Fatalf("back: same host, gone cleared, new address: %+v", p)
	}
}
