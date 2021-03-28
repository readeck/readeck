package acls

import (
	"embed"
	"errors"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	defaultrolemanager "github.com/casbin/casbin/v2/rbac/default-role-manager"
	"github.com/casbin/casbin/v2/util"
)

//go:embed config/*
var confFiles embed.FS

var enforcer *casbin.Enforcer

// Check performs the rule enforcment for a given user, path and action.
func Check(group, path, act string) (bool, error) {
	return enforcer.Enforce(group, path, act)
}

// GetRoles returns the implicit roles for a given group
func GetRoles(group string) ([]string, error) {
	return enforcer.GetImplicitRolesForUser(group)
}

func init() {
	var err error
	enforcer, err = newEnforcer()
	if err != nil {
		panic(err)
	}
}

func newEnforcer() (*casbin.Enforcer, error) {
	c, err := confFiles.ReadFile("config/model.ini")
	if err != nil {
		return nil, err
	}
	m, err := model.NewModelFromString(string(c))
	if err != nil {
		return nil, err
	}

	policy, err := confFiles.ReadFile("config/policy.conf")
	if err != nil {
		return nil, err
	}
	sa := newAdapter(string(policy))
	e, _ := casbin.NewEnforcer()
	err = e.InitWithModelAndAdapter(m, sa)
	if err != nil {
		return nil, err
	}

	rm := e.GetRoleManager()
	rm.(*defaultrolemanager.RoleManager).AddMatchingFunc("KeyMatch2", util.KeyMatch2)

	return e, err
}

type adapter struct {
	contents string
}

func newAdapter(contents string) *adapter {
	return &adapter{
		contents: contents,
	}
}

func (sa *adapter) LoadPolicy(model model.Model) error {
	if sa.contents == "" {
		return errors.New("invalid line, line cannot be empty")
	}
	lines := strings.Split(sa.contents, "\n")
	for _, str := range lines {
		if str == "" {
			continue
		}
		persist.LoadPolicyLine(str, model)
	}

	return nil
}

func (sa *adapter) SavePolicy(_ model.Model) error {
	return errors.New("not implemented")
}

func (sa *adapter) AddPolicy(_ string, _ string, _ []string) error {
	return errors.New("not implemented")
}

func (sa *adapter) RemovePolicy(_ string, _ string, _ []string) error {
	return errors.New("not implemented")
}

func (sa *adapter) RemoveFilteredPolicy(_ string, _ string, _ int, _ ...string) error {
	return errors.New("not implemented")
}
