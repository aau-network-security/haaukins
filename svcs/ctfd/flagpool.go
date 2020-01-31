// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package ctfd

import (
	"sync"

	"github.com/aau-network-security/haaukins"
	"github.com/aau-network-security/haaukins/store"
)

type FlagPool struct {
	m    sync.RWMutex
	ids  map[int]*activeFlagConfig
	tags map[store.Tag]*activeFlagConfig
}

type activeFlagConfig struct {
	CTFdFlag       haaukins.Flag
	CTFdIdentifier int
	store.FlagConfig
}

func (afc *activeFlagConfig) IsStatic() bool {
	return afc.FlagConfig.StaticValue != ""
}

func NewFlagPool() *FlagPool {
	return &FlagPool{
		ids:  map[int]*activeFlagConfig{},
		tags: map[store.Tag]*activeFlagConfig{},
	}
}

func (fp *FlagPool) AddFlag(conf store.FlagConfig, cid int) haaukins.Flag {
	fp.m.Lock()
	defer fp.m.Unlock()

	var flag haaukins.Flag
	flag, _ = haaukins.NewFlagStatic(conf.StaticValue)
	if conf.StaticValue == "" {
		flag = haaukins.NewFlagShort()
	}

	fconf := activeFlagConfig{flag, cid, conf}

	fp.tags[conf.Tag] = &fconf
	fp.ids[cid] = &fconf

	return flag
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

func (fp *FlagPool) GetFlagByTag(t store.Tag) (haaukins.Flag, error) {
	fp.m.RLock()
	defer fp.m.RUnlock()

	conf, ok := fp.tags[t]
	if !ok {
		return nil, store.UnknownChallengeErr
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

func (fp *FlagPool) TranslateFlagForTeam(t store.Team, cid int, value string) string {
	fp.m.RLock()
	defer fp.m.RUnlock()

	chal, ok := fp.ids[cid]
	if !ok {
		return ""
	}

	if chal.IsStatic() {
		return chal.CTFdFlag.String()
	}

	if err := t.IsCorrectFlag(chal.Tag, value); err != nil {
		return ""
	}

	return chal.CTFdFlag.String()
}
