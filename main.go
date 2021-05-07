package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"text/template"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiserver/pkg/apis/audit"
)

var (
	auditLog       = flag.String("f", "/tmp/audit/audit.log", "Audit log file to process")
	serviceAccount = flag.String("s", "system:serviceaccount:tekton-pipelines:tekton-pipelines-controller", "ServiceAccount name to filter for")
	namespace      = flag.String("ns", "tekton-pipelines", "Special system namespace")
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

		if evt.User.Username != *serviceAccount {
			continue
		}

		if evt.ObjectRef == nil {
			continue
		}
		i := item{
			gvr: gvr{
				APIGroup:    evt.ObjectRef.APIGroup,
				APIVersion:  evt.ObjectRef.APIVersion,
				Resource:    evt.ObjectRef.Resource,
				Subresource: evt.ObjectRef.Subresource,
			},
			Verb: evt.Verb,
		}
		if evt.ObjectRef.Namespace == *namespace {
			nm[i] = struct{}{}
		} else {
			m[i] = struct{}{}
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
		sort.Strings(vbs)
		apigroup := fmt.Sprintf("%s/%s", gvr.APIGroup, gvr.APIVersion)
		apigroup = strings.TrimPrefix(apigroup, "/")
		pr := rbacv1.PolicyRule{
			Verbs:     vbs,
			APIGroups: []string{apigroup},
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

type gvr struct{ APIGroup, APIVersion, Resource, Subresource string }

type item struct {
	gvr
	Verb string
}
