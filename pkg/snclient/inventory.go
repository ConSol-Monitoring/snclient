package snclient

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// InventoryCacheDuration sets the cache duration for inventory.
	InventoryCacheDuration = 10 * time.Second
)

type Inventory map[string]any

type InvCache struct {
	mutex      sync.Mutex
	cond       *sync.Cond
	inventory  *Inventory
	lastUpdate time.Time
	updating   bool
}

func NewInvCache() *InvCache {
	c := &InvCache{}
	c.cond = sync.NewCond(&c.mutex)

	return c
}

func (ic *InvCache) Get(ctx context.Context, snc *Agent) *Inventory {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	// if fresh, return immediately
	if ic.inventory != nil && time.Since(ic.lastUpdate) < InventoryCacheDuration {
		return ic.inventory
	}

	// if another goroutine is already updating, wait
	if ic.updating {
		for ic.updating {
			ic.cond.Wait()
		}

		return ic.inventory
	}

	// run the update
	ic.updating = true
	ic.mutex.Unlock()

	// do the update outside of lock
	newInv := snc.buildInventory(ctx, nil)

	ic.mutex.Lock()
	defer ic.cond.Broadcast() // wake all waiting goroutines
	ic.updating = false

	ic.inventory = newInv
	ic.lastUpdate = time.Now()

	return ic.inventory
}

func (snc *Agent) getInventoryEntry(ctx context.Context, checkName string) (listData []map[string]string, err error) {
	checkName = strings.TrimPrefix(checkName, "check_")
	rawInv := snc.GetInventory(ctx, []string{checkName})
	invInterface, ok := rawInv["inventory"]
	if !ok {
		return nil, fmt.Errorf("check %s not found in inventory", checkName)
	}

	inv, ok := invInterface.(*Inventory)
	if !ok {
		return nil, fmt.Errorf("unexpected inventory type: %T", inv)
	}

	list, ok := (*inv)[checkName]
	if !ok {
		return nil, fmt.Errorf("unexpected inventory entry type: %T", list)
	}

	data, ok := list.([]map[string]string)
	if !ok {
		return nil, fmt.Errorf("could not build inventory for %s", checkName)
	}

	return data, nil
}

func (snc *Agent) buildInventory(ctx context.Context, modules []string) *Inventory {
	scripts := make([]string, 0)
	inventory := make(Inventory)

	keys := make([]string, 0)
	for k := range AvailableChecks {
		keys = append(keys, k)
	}
	for k := range snc.runSet.cmdAliases {
		keys = append(keys, k)
	}
	for k := range snc.runSet.cmdWraps {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		check, _ := snc.getCheck(k, false)
		handler := check.Handler()
		meta := handler.Build()
		if !meta.isImplemented(runtime.GOOS) {
			log.Debugf("skipping inventory for unimplemented (%s) check: %s / %s", runtime.GOOS, k, check.Name)

			continue
		}
		switch meta.hasInventory {
		case NoInventory:
			// skipped
		case ListInventory:
			name := strings.TrimPrefix(check.Name, "check_")
			if len(modules) > 0 && (!slices.Contains(modules, name)) {
				continue
			}
			meta.output = "inventory_json"
			meta.filter = ConditionList{{isNone: true}}
			data, err := handler.Check(ctx, snc, meta, []Argument{})
			if err != nil && (data == nil || data.Raw == nil) {
				log.Tracef("inventory %s returned error: %s", check.Name, err.Error())

				continue
			}

			inventory[name] = data.Raw.listData
		case NoCallInventory:
			name := strings.TrimPrefix(check.Name, "check_")
			if len(modules) > 0 && !slices.Contains(modules, name) {
				continue
			}
			inventory[name] = []any{}
		case ScriptsInventory:
			scripts = append(scripts, check.Name)
		}
	}

	if len(modules) == 0 || slices.Contains(modules, "scripts") {
		inventory["scripts"] = scripts
	}

	if len(modules) == 0 || slices.Contains(modules, "exporter") {
		inventory["exporter"] = snc.listExporter()
	}

	return &inventory
}
