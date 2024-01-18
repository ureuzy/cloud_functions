package role_recommender

import (
	"strings"
)

type Hoge struct {
	Resource
	RecommendsMap RecommendsMap
}
type Recommend struct {
	Role   string
	Member string
}

type Recommends []Recommend

type RecommendsMap map[string]Recommends

type Resource interface {
	GetName() string
	GetDisplayName() string
}

type SheetData [][]interface{}

func (r *Recommends) toRoleList() []string {
	var result []string
	for _, v := range *r {
		result = append(result, v.Role)
	}
	return result
}

func (r *Recommends) groupByMember() RecommendsMap {
	group := make(map[string]Recommends)
	for _, recommend_ := range *r {
		if v, exists := group[recommend_.Member]; exists {
			v = append(v, recommend_)
		}
		group[recommend_.Member] = append(group[recommend_.Member], recommend_)
	}
	return group
}

func (r *Hoge) toSheetData() SheetData {
	var sheet [][]interface{}
	for k, v := range (*r).RecommendsMap {
		col := make([]interface{}, 4)
		roles := strings.Join(v.toRoleList(), "\n")
		type_ := strings.Split(r.Resource.GetName(), "/")[0]
		for i, s := range []string{r.Resource.GetDisplayName(), type_, k, roles} {
			col[i] = s
		}
		sheet = append(sheet, col)
	}
	return sheet
}

func (r *SheetData) merge(data SheetData) {
	for _, v := range data {
		*r = append(*r, v)
	}
}
