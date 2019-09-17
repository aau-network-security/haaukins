// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package ctfd

import (
	"sync"

	"github.com/aau-network-security/haaukins/store"
	"github.com/google/uuid"
)

type FlagPool struct {
	m    sync.RWMutex
	ids  map[int]*activeFlagConfig
	tags map[store.Tag]*activeFlagConfig
}

type activeFlagConfig struct {
	CTFdFlag       string
	CTFdIdentifier int
	store.FlagConfig
}

func (afc *activeFlagConfig) IsStatic() bool {
	return afc.FlagConfig.Static != ""
}

func NewFlagPool() *FlagPool {
	return &FlagPool{
		ids:  map[int]*activeFlagConfig{},
		tags: map[store.Tag]*activeFlagConfig{},
	}
}

func (fp *FlagPool) AddFlag(flag store.FlagConfig, cid int) string {
	fp.m.Lock()
	defer fp.m.Unlock()

	value := flag.Static
	if value == "" {
		value = uuid.New().String()
	}

	fconf := activeFlagConfig{value, cid, flag}

	fp.tags[flag.Tag] = &fconf
	fp.ids[cid] = &fconf

	return value
}

func (fp *FlagPool) GetIdentifierByTag(t store.Tag) (int, error) {
	fp.m.RLock()
	defer fp.m.RUnlock()

	conf, ok := fp.tags[t]
	if !ok {
		return 0, store.UnknownChallengeErr
	}

	return conf.CTFdIdentifier, nil
}

func (fp *FlagPool) GetFlagByTag(t store.Tag) (string, error) {
	fp.m.RLock()
	defer fp.m.RUnlock()

	conf, ok := fp.tags[t]
	if !ok {
		return "", store.UnknownChallengeErr
	}

	return conf.CTFdFlag, nil
}

func (fp *FlagPool) GetTagByIdentifier(id int) (store.Tag, error) {
	fp.m.RLock()
	defer fp.m.RUnlock()

	conf, ok := fp.ids[id]
	if !ok {
		return "", store.UnknownChallengeErr
	}

	return conf.Tag, nil
}

func (fp *FlagPool) TranslateFlagForTeam(t *store.Team, cid int, value string) string {
	fp.m.RLock()
	defer fp.m.RUnlock()

	chal, ok := fp.ids[cid]
	if !ok {
		return "Challenge cannot be retrievend from flag pool ! "
	}

	if err := t.IsCorrectFlag(chal.Tag, value); err != nil {
		return "Error happened on checking flag ! "+ err.Error()
	}

	return chal.CTFdFlag
}
