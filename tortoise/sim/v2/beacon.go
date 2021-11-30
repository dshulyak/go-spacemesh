package sim

import "math/rand"

var _ StateMachine = (*Beacon)(nil)

// Beacon outputs beacon at the last layer in epoch.
type Beacon struct {
	rng *rand.Rand
}

// OnEvent ...
func (b *Beacon) OnEvent(event Event) []Event {
	switch ev := event.(type) {
	case EventLayerEnd:
		if ev.LayerID.GetEpoch() == ev.LayerID.Add(1).GetEpoch() {
			break
		}
		beacon := make([]byte, 32)
		b.rng.Read(beacon)
		return []Event{
			EventBeacon{
				EpochID: ev.LayerID.GetEpoch(),
				Beacon:  beacon,
			},
		}
	}
	return nil
}
