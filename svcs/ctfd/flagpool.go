package ctfd

import (
	"sync"

	"github.com/aau-network-security/go-ntp/store"
)

type flagPool struct {
	m    sync.RWMutex
	ids  map[int]*activeFlagConfig
	tags map[store.Tag]*activeFlagConfig
}

type activeFlagConfig struct {
	CTFdFlag       string
	CTFdIdentifier int
	store.FlagConfig
}

func NewFlagPool() *flagPool {
	return &flagPool{
		ids:  map[int]*activeFlagConfig{},
		tags: map[store.Tag]*activeFlagConfig{},
	}
}

func (fp *flagPool) AddFlag(conf store.FlagConfig, cid int, value string) {
	fp.m.Lock()
	defer fp.m.Unlock()

	fconf := activeFlagConfig{value, cid, conf}

	fp.tags[conf.Tag] = &fconf
	fp.ids[cid] = &fconf
}

func (fp *flagPool) GetIdentifierByTag(t store.Tag) (int, error) {
	fp.m.RLock()
	defer fp.m.RUnlock()

	conf, ok := fp.tags[t]
	if !ok {
		return 0, store.UnknownChallengeErr
	}

	return conf.CTFdIdentifier, nil
}

func (fp *flagPool) GetFlagByTag(t store.Tag) (string, error) {
	fp.m.RLock()
	defer fp.m.RUnlock()

	conf, ok := fp.tags[t]
	if !ok {
		return "", store.UnknownChallengeErr
	}

	return conf.CTFdFlag, nil
}

func (fp *flagPool) GetTagByIdentifier(id int) (store.Tag, error) {
	fp.m.RLock()
	defer fp.m.RUnlock()

	conf, ok := fp.ids[id]
	if !ok {
		return "", store.UnknownChallengeErr
	}

	return conf.Tag, nil
}

func (fp *flagPool) TranslateFlagForTeam(t store.Team, cid int, value string) string {
	fp.m.RLock()
	defer fp.m.RUnlock()

	chal, ok := fp.ids[cid]
	if !ok {
		return ""
	}

	if static := chal.Static != ""; static {
		return value
	}

	if err := t.IsCorrectFlag(chal.Tag, value); err != nil {
		return ""
	}

	return chal.CTFdFlag
}
