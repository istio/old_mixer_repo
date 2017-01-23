// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package memQuota

// Implements a rolling window that allows N units to be allocated per rolling time interval.
// Time is abstracted in terms of ticks, provided by the caller, decoupling the
// implementation from real-time, enabling much easier testing and more flexibility.
type rollingWindow struct {
	// one slot per tick in the window, tracking consumption for that tick
	slots []int64

	// slot where consumption for the currentSlotTick is recorded
	currentSlot int

	// the tick count associated with the current slot
	currentSlotTick int32

	// the total # of units currently available in the window
	avail int64
}

func newRollingWindow(limit int64, ticksInWindow int32) *rollingWindow {
	return &rollingWindow{
		avail: limit,
		slots: make([]int64, ticksInWindow),
	}
}

func (w *rollingWindow) alloc(amount int64, currentTick int32) bool {
	// how many ticks has time marched forward since our last time here?
	behind := int(currentTick - w.currentSlotTick)
	if behind > len(w.slots) {
		behind = len(w.slots)
	}

	// reclaim any units that are now outside of the window
	for i := 0; i < behind; i++ {
		index := (w.currentSlot + 1 + i) % len(w.slots)
		w.avail += w.slots[index]
		w.slots[index] = 0
	}

	if amount > w.avail {
		// not enough room
		return false
	}

	// now record the units being allocated
	w.currentSlot = (w.currentSlot + behind) % len(w.slots)
	w.currentSlotTick = currentTick
	w.slots[w.currentSlot] += amount
	w.avail -= amount

	return true
}

func (w *rollingWindow) available() int64 {
	return w.avail
}
