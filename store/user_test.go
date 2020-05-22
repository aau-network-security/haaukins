// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package store_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/aau-network-security/haaukins/store"
)

func TestUser(t *testing.T) {
	u, err := store.NewUser("tkp", "", "", "", "test123")
	if err != nil {
		t.Fatalf("expected no error when creating user struct, got: %s", err)
	}

	if ok := u.IsCorrectPassword("test123"); !ok {
		t.Fatalf("expected no error when comparing passwords")
	}

	if ok := u.IsCorrectPassword("testtttttttt123"); ok {
		t.Fatalf("expected error when comparing incorrect passwords")
	}
}

func TestUserStore(t *testing.T) {
	var ran bool
	var count int

	us := store.NewUserStore([]store.User{}, func(us []store.User) error {
		ran = true
		count = len(us)
		return nil
	})

	if n := len(us.ListUsers()); n != 0 {
		t.Fatalf("unexpected amount of users, exptected: 0, got: %d", n)
	}

	u, err := store.NewUser("tkp", "", "", "", "test123")
	if err != nil {
		t.Fatalf("expected no error when creating user struct, got: %s", err)
	}

	if err := us.CreateUser(u); err != nil {
		t.Fatalf("expected no error when storing user struct, got: %s", err)
	}

	if !ran {
		t.Fatalf("expected hook to have been run")
	}

	if count != 1 {
		t.Fatalf("expected hook to have been run with one user")
	}

	if err := us.DeleteUserByUsername(u.Username); err != nil {
		t.Fatalf("expected no error when deleting user struct, got: %s", err)
	}

	if count != 0 {
		t.Fatalf("expected hook to have been run with zero users")
	}

	u2, err := store.NewUser("tkp", "", "", "", "test123")
	if err != nil {
		t.Fatalf("expected no error when creating user struct, got: %s", err)
	}

	ran = false
	if err := us.DeleteUserByUsername(u2.Username); err == nil {
		t.Fatalf("expected error when deleting unknown user, got: nil")
	}

	if ran {
		t.Fatalf("expected hook not to have been run")
	}
}

func TestSignupKeyStore(t *testing.T) {
	var ran bool
	var count int

	ss := store.NewSignupKeyStore([]store.SignupKey{}, func(ss []store.SignupKey) error {
		ran = true
		count = len(ss)
		return nil
	})

	if n := len(ss.ListSignupKeys()); n != 0 {
		t.Fatalf("unexpected amount of signup keys, expected: 0, got: %d", n)
	}

	k := store.NewSignupKey()
	if err := ss.CreateSignupKey(k); err != nil {
		t.Fatalf("expected no error when storing signup key struct, got: %s", err)
	}

	if !ran {
		t.Fatalf("expected hook to have been run")
	}

	if count != 1 {
		t.Fatalf("expected hook to have been run with one user")
	}

	if err := ss.DeleteSignupKey(k); err != nil {
		t.Fatalf("expected no error when deleting signup key, got: %s", err)
	}

	if count != 0 {
		t.Fatalf("expected hook to have been run with zero users")
	}
}

func TestUserFile(t *testing.T) {
	f, err := ioutil.TempFile("", "users.yml")
	if err != nil {
		t.Fatalf("expected no error when creating temporary file")
	}
	defer os.Remove(f.Name())

	uf, err := store.NewUserFile(f.Name())
	if err != nil {
		t.Fatalf("expected no error when creating userfile")
	}

	u, err := store.NewUser("tkp", "", "", "", "test123")
	if err != nil {
		t.Fatalf("expected no error when creating user struct, got: %s", err)
	}

	if err := uf.CreateUser(u); err != nil {
		t.Fatalf("expected no error when storing user struct, got: %s", err)
	}

	n := len(uf.ListUsers())
	uf, err = store.NewUserFile(f.Name())
	if err != nil {
		t.Fatalf("expected no error when creating userfile")
	}

	if n != len(uf.ListUsers()) {
		t.Fatalf("expected new userfile to have same amount of users as previous one")
	}
}
