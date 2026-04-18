package app

import (
	"errors"
	"testing"
)

type mockSkillsManager struct {
	checkCalled  bool
	updateCalled bool
	checkErr     error
	updateErr    error
}

func (m *mockSkillsManager) Check() error {
	m.checkCalled = true
	return m.checkErr
}

func (m *mockSkillsManager) Update() error {
	m.updateCalled = true
	return m.updateErr
}

func TestAISkillsCheck_Success(t *testing.T) {
	mgr := &mockSkillsManager{}
	a := New(Deps{SkillsManager: mgr})

	err := a.AISkillsCheck()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mgr.checkCalled {
		t.Error("expected Check to be called")
	}
}

func TestAISkillsCheck_Error(t *testing.T) {
	mgr := &mockSkillsManager{checkErr: errors.New("check failed")}
	a := New(Deps{SkillsManager: mgr})

	err := a.AISkillsCheck()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "check failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAISkillsCheck_NilManager(t *testing.T) {
	a := New(Deps{})

	err := a.AISkillsCheck()
	if err == nil {
		t.Fatal("expected error for nil manager, got nil")
	}
	if err.Error() != "skills manager not available (is npx installed?)" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAISkillsUpdate_Success(t *testing.T) {
	mgr := &mockSkillsManager{}
	a := New(Deps{SkillsManager: mgr})

	err := a.AISkillsUpdate()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mgr.updateCalled {
		t.Error("expected Update to be called")
	}
}

func TestAISkillsUpdate_Error(t *testing.T) {
	mgr := &mockSkillsManager{updateErr: errors.New("update failed")}
	a := New(Deps{SkillsManager: mgr})

	err := a.AISkillsUpdate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "update failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAISkillsUpdate_NilManager(t *testing.T) {
	a := New(Deps{})

	err := a.AISkillsUpdate()
	if err == nil {
		t.Fatal("expected error for nil manager, got nil")
	}
	if err.Error() != "skills manager not available (is npx installed?)" {
		t.Errorf("unexpected error: %v", err)
	}
}
