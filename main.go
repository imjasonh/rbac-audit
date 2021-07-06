package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"text/template"

	authv1 "k8s.io/api/authorization/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiserver/pkg/apis/audit"
)

var (
	auditLog       = flag.String("f", "/tmp/audit/audit.log", "Audit log file to process")
	serviceAccount = flag.String("serviceaccount", "tekton-pipelines-controller", "ServiceAccount name to filter for")
	namespace      = flag.String("namespace", "tekton-pipelines", "Special system namespace")
	verbose        = flag.Bool("v", false, "If true, print new items as found")
)

var tmpl = template.Must(template.New("").Parse(`apiVersion: rbac.authorization.k8s.io/v1
kind: {{ .Kind }}
metadata:
  name: {{ .Name }}{{ if .Namespace }}
  namespace: {{ .Namespace }}{{ end }}
spec:
  rules:{{ range .Rules }}
  - apiGroups: ['{{ index .APIGroups 0 }}']
    resources: ['{{ index .Resources 0 }}']
    verbs:     [{{ range $idx, $v := .Verbs }}{{ if $idx }}, {{ end }}'{{ $v }}'{{ end }}]
{{ end }}
`))

type data struct {
	Name, Namespace, Kind string
	Rules                 []rbacv1.PolicyRule
}

func main() {
	flag.Parse()
	targetUser := fmt.Sprintf("system:serviceaccount:%s:%s", *namespace, *serviceAccount)

	f, err := os.Open(*auditLog)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	m := items{}
	nm := items{}
	dec := json.NewDecoder(f)
	for {
		var evt audit.Event
		if err := dec.Decode(&evt); err == io.EOF {
			break
		} else if err != nil {
			log.Println(err)
			log.Println("---")
			continue
		}
		if evt.ObjectRef == nil {
			continue
		}

		var i item
		if evt.ObjectRef.Resource == "subjectaccessreviews" {
			var sar authv1.SubjectAccessReview
			if err := json.Unmarshal(evt.RequestObject.Raw, &sar); err != nil {
				log.Println(err)
				log.Println("---")
				continue
			}
			if sar.Spec.User != targetUser {
				continue
			}
			i = item{
				gvr: gvr{
					APIGroup:    sar.Spec.ResourceAttributes.Group,
					Resource:    sar.Spec.ResourceAttributes.Resource,
					Subresource: sar.Spec.ResourceAttributes.Subresource,
				},
				Verb: sar.Spec.ResourceAttributes.Verb,
				sar:  true,
			}
		} else {
			if evt.User.Username != targetUser {
				continue
			}
			i = item{
				gvr: gvr{
					APIGroup:    evt.ObjectRef.APIGroup,
					Resource:    evt.ObjectRef.Resource,
					Subresource: evt.ObjectRef.Subresource,
				},
				Verb: evt.Verb,
			}
		}

		if evt.ObjectRef.Namespace == *namespace {
			if _, found := nm[i]; found {
				continue
			}
			if *verbose {
				log.Printf("found namespaced request for: %s %s %s %s (sar=%d)", i.Verb, i.APIGroup, i.Resource, i.Subresource, i.sar)
			}
			nm[i] = struct{}{}
		} else {
			if _, found := m[i]; found {
				continue
			}
			m[i] = struct{}{}
			if *verbose {
				log.Printf("found cluster request for: %s %s %s %s (sar=%d)", i.Verb, i.APIGroup, i.Resource, i.Subresource, i.sar)
			}
		}
	}

	if err := tmpl.Execute(os.Stdout, data{
		Name:      "generated-minimal-role",
		Namespace: *namespace,
		Kind:      "Role",
		Rules:     nm.toPolicyRules(),
	}); err != nil {
		log.Fatal(err)
	}
	fmt.Println("---")
	if err := tmpl.Execute(os.Stdout, data{
		Name:  "generated-minimal-cluster-role",
		Kind:  "ClusterRole",
		Rules: m.toPolicyRules(),
	}); err != nil {
		log.Fatal(err)
	}
}

type items map[item]struct{}

func (i items) toPolicyRules() []rbacv1.PolicyRule {
	bygvr := map[gvr]map[string]struct{}{}
	for k := range i {
		if bygvr[k.gvr] == nil {
			bygvr[k.gvr] = map[string]struct{}{}
		}
		bygvr[k.gvr][k.Verb] = struct{}{}
	}
	prs := []rbacv1.PolicyRule{}
	for gvr, verbs := range bygvr {
		vbs := []string{}
		for k := range verbs {
			vbs = append(vbs, k)
		}
		sortVerbs(vbs)
		pr := rbacv1.PolicyRule{
			Verbs:     vbs,
			APIGroups: []string{gvr.APIGroup},
			Resources: []string{gvr.Resource},
		}
		if gvr.Subresource != "" {
			pr.Resources = []string{gvr.Resource + "/" + gvr.Subresource}
		}
		prs = append(prs, pr)
	}
	sort.Slice(prs, func(i, j int) bool {
		if prs[i].APIGroups[0] != prs[j].APIGroups[0] {
			return prs[i].APIGroups[0] < prs[j].APIGroups[0]
		}
		return prs[i].Resources[0] < prs[j].Resources[0]
	})
	return prs
}

type gvr struct{ APIGroup, Resource, Subresource string }

type item struct {
	gvr
	Verb string
	sar  bool
}

var order = map[string]int{
	"get":              1,
	"list":             2,
	"watch":            3,
	"patch":            4,
	"update":           5,
	"create":           6,
	"delete":           7,
	"deletecollection": 8,
}

func sortVerbs(verbs []string) {
	sort.Slice(verbs, func(i, j int) bool {
		a, b := verbs[i], verbs[j]
		ao, bo := order[a], order[b]
		// Sort unknown orders to the end.
		if ao == 0 {
			ao = 1000
		}
		if bo == 0 {
			bo = 1000
		}
		// If both are unknown, alphabetical.
		if ao == bo {
			return a < b
		}
		return ao < bo
	})
}
