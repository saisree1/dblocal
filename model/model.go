package model

type DbTable struct {
	Id   int `orm:"column(id);auto"`
	Name int `orm:"column(name);size(25)"`
}
