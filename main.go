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

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apiserver/pkg/apis/audit"
)

var (
	auditLog       = flag.String("f", "/tmp/audit/audit.log", "Audit log file to process")
	serviceAccount = flag.String("s", "system:serviceaccount:tekton-pipelines:tekton-pipelines-controller", "ServiceAccount name to filter for")
	namespace      = flag.String("ns", "tekton-pipelines", "Special system namespace")
)

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

	r := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: *namespace,
			Name:      "generated-minimal-role",
		},
		Rules: nm.toPolicyRules(),
	}
	if err := kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, nil, nil).Encode(r, os.Stdout); err != nil {
		log.Fatal(err)
	}
	fmt.Println("---")
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "generated-minimal-cluster-role",
		},
		Rules: m.toPolicyRules(),
	}
	if err := kjson.NewYAMLSerializer(kjson.DefaultMetaFactory, nil, nil).Encode(cr, os.Stdout); err != nil {
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
