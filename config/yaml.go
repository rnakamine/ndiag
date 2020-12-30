package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/k1LoW/glyph"
	"github.com/k1LoW/tbls/dict"
)

func (d *Config) UnmarshalYAML(data []byte) error {
	raw := struct {
		Name            string             `yaml:"name"`
		Desc            string             `yaml:"desc,omitempty"`
		DocPath         string             `yaml:"docPath"`
		DescPath        string             `yaml:"descPath"`
		Graph           *Graph             `yaml:"graph,omitempty"`
		HideDiagrams    bool               `yaml:"hideDiagrams"`
		HideLayers      bool               `yaml:"hideLayers"`
		HideRealNodes   bool               `yaml:"hideRealNodes"`
		HideLabelGroups bool               `yaml:"hideLabelGroups"`
		Diagrams        []*Diagram         `yaml:"diagrams"`
		Nodes           []*Node            `yaml:"nodes"`
		Networks        []interface{}      `yaml:"networks"`
		Relations       []interface{}      `yaml:"relations"`
		Dict            *dict.Dict         `yaml:"dict,omitempty"`
		BaseColor       string             `yaml:"baseColor,omitempty"`
		TextColor       string             `yaml:"textColor,omitempty"`
		CustomIcons     []*glyph.Blueprint `yaml:"customIcons,omitempty"`
	}{}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	d.Name = raw.Name
	d.Desc = raw.Desc
	d.DocPath = raw.DocPath
	d.DescPath = raw.DescPath
	if raw.Graph != nil {
		d.Graph = raw.Graph
	}
	d.HideDiagrams = raw.HideDiagrams
	d.HideLayers = raw.HideLayers
	d.HideRealNodes = raw.HideRealNodes
	d.HideLabelGroups = raw.HideLabelGroups
	d.Diagrams = raw.Diagrams
	d.Nodes = raw.Nodes
	if raw.Dict != nil {
		d.Dict = raw.Dict
	}
	d.BaseColor = raw.BaseColor
	d.TextColor = raw.TextColor
	d.CustomIcons = raw.CustomIcons

	for _, rel := range raw.Networks {
		rel, err := parseRelation(RelationTypeNetwork, rel)
		if err != nil {
			return err
		}
		d.rawRelations = append(d.rawRelations, rel)
	}

	for _, rel := range raw.Relations {
		rel, err := parseRelation(RelationTypeDefault, rel)
		if err != nil {
			return err
		}
		d.rawRelations = append(d.rawRelations, rel)
	}
	return nil
}

func (n *Node) UnmarshalYAML(data []byte) error {
	raw := struct {
		Name        string       `yaml:"name"`
		Desc        string       `yaml:"desc"`
		Match       string       `yaml:"match,omitempty"`
		MatchRegexp string       `yaml:"matchRegexp,omitempty"`
		Components  []string     `yaml:"components,omitempty"`
		Clusters    []string     `yaml:"clusters,omitempty"`
		Metadata    NodeMetadata `yaml:"metadata,omitempty"`
	}{}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	if sepContains(raw.Name) {
		return fmt.Errorf("a node's name cannot contain unescaped '%s': %s ", Sep, raw.Name)
	}

	n.Name = raw.Name
	n.Match = raw.Match
	n.MatchRegexp = raw.MatchRegexp
	if n.Match == "" {
		n.Match = n.Name
	}
	if n.MatchRegexp == "" {
		n.nameRe = regexp.MustCompile(fmt.Sprintf("^%s$", strings.Replace(n.Match, "*", ".+", -1)))
	} else {
		n.nameRe = regexp.MustCompile(n.MatchRegexp)
	}

	n.Desc = raw.Desc
	n.rawComponents = raw.Components
	n.rawClusters = raw.Clusters
	n.Metadata = raw.Metadata
	return nil
}

func parseRelation(relType *RelationType, rel interface{}) (*rawRelation, error) {
	components := []string{}
	labels := []string{}
	switch v := rel.(type) {
	case []interface{}:
		for _, r := range v {
			components = append(components, r.(string))
		}
		if len(components) < 2 {
			return nil, fmt.Errorf("invalid %s format: %s", relType.Name, v)
		}
		rel := &rawRelation{
			Type:       relType,
			Components: components,
			Attrs:      relType.Attrs,
		}
		rel.Labels = []string{rel.Id()}
		return rel, nil
	case map[string]interface{}:
		var (
			id string
		)
		idi, ok := v["id"]
		if ok {
			id = idi.(string)
		} else {
			id = ""
		}
		ri, ok := v[relType.ComponentsKey]
		if !ok {
			return nil, fmt.Errorf("invalid %s format: %s", relType.Name, v)
		}
		for _, r := range ri.([]interface{}) {
			components = append(components, r.(string))
		}
		if len(components) < 2 {
			return nil, fmt.Errorf("invalid %s format: %s", relType.Name, v)
		}
		typei, ok := v["type"]
		if ok {
			switch typei.(string) {
			case "network":
				relType = RelationTypeNetwork
			default:
				return nil, fmt.Errorf("invalid %s format: %s", relType.Name, v)
			}
		}
		ti, ok := v["labels"]
		if ok {
			for _, t := range ti.([]interface{}) {
				labels = append(labels, t.(string))
			}
		}
		if len(labels) == 0 {
			labels = []string{id}
		}
		attrs := []*Attr{}
		attrsi, ok := v["attrs"]
		if ok {
			for k, v := range attrsi.(map[string]interface{}) {
				attrs = append(attrs, &Attr{
					Key:   k,
					Value: v.(string),
				})
			}
		}
		sort.Slice(attrs, func(i, j int) bool {
			if attrs[i].Key == attrs[j].Key {
				return attrs[i].Value < attrs[j].Value
			}
			return attrs[i].Key < attrs[j].Key
		})
		attrs = append(relType.Attrs, attrs...)

		return &rawRelation{
			relationId: id,
			Type:       relType,
			Components: components,
			Labels:     labels,
			Attrs:      attrs,
		}, nil
	default:
		return nil, fmt.Errorf("invalid relation format: %s", v)
	}
}
