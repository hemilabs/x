package keygen

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hemilabs/x/tss-lib/v2/test"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// partyMetrics tracks cumulative compute time per party across all
// Start() and Update() calls during a single keygen ceremony.
type partyMetrics struct {
	mu      sync.Mutex
	elapsed []time.Duration
}

func newPartyMetrics(n int) *partyMetrics {
	return &partyMetrics{elapsed: make([]time.Duration, n)}
}

func (pm *partyMetrics) add(idx int, d time.Duration) {
	pm.mu.Lock()
	pm.elapsed[idx] += d
	pm.mu.Unlock()
}

func (pm *partyMetrics) median() time.Duration {
	s := make([]time.Duration, len(pm.elapsed))
	copy(s, pm.elapsed)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[len(s)/2]
}

func (pm *partyMetrics) max() time.Duration {
	var m time.Duration
	for _, d := range pm.elapsed {
		if d > m {
			m = d
		}
	}
	return m
}

func BenchmarkKeygen(b *testing.B) {
	setUp("error")

	fixtures, _, err := LoadKeygenTestFixtures(testParticipants)
	if err != nil {
		b.Skip("fixtures not found; run TestE2EConcurrentAndSaveFixtures first")
	}

	// Protocol enforces unique DLN params (h1j, h2j) per party, so pre-params
	// cannot be reused. Parties beyond len(fixtures) generate fresh safe primes
	// (~2 min each). Sizes <= 5 use only pre-computed fixtures.
	for _, n := range []int{3, 5, 7, 11, 23, 35, 51, 67, 101} {
		n := n
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			var medTotal, maxTotal float64
			for i := 0; i < b.N; i++ {
				pm := runKeygen(b, n, n/2, fixtures)
				medTotal += float64(pm.median().Nanoseconds())
				maxTotal += float64(pm.max().Nanoseconds())
			}
			b.ReportMetric(medTotal/float64(b.N), "median-party-ns/op")
			b.ReportMetric(maxTotal/float64(b.N), "max-party-ns/op")
		})
	}
}

func runKeygen(b *testing.B, n, threshold int, fixtures []LocalPartySaveData) *partyMetrics {
	b.Helper()

	pIDs := tss.GenerateTestPartyIDs(n)
	p2pCtx := tss.NewPeerContext(pIDs)
	parties := make([]*LocalParty, 0, n)
	pm := newPartyMetrics(n)

	errCh := make(chan *tss.Error, n)
	outCh := make(chan tss.Message, n*n)
	endCh := make(chan *LocalPartySaveData, n)

	for i := 0; i < n; i++ {
		params := tss.NewParameters(tss.S256(), p2pCtx, pIDs[i], n, threshold)
		params.SetNoProofMod()
		params.SetNoProofFac()
		var P *LocalParty
		if i < len(fixtures) {
			P = NewLocalParty(params, outCh, endCh, fixtures[i].LocalPreParams).(*LocalParty)
		} else {
			P = NewLocalParty(params, outCh, endCh).(*LocalParty)
		}
		parties = append(parties, P)
		go func(P *LocalParty, idx int) {
			start := time.Now()
			if err := P.Start(); err != nil {
				errCh <- err
				return
			}
			pm.add(idx, time.Since(start))
		}(P, i)
	}

	var ended int32
	for {
		select {
		case err := <-errCh:
			b.Fatal(err)
			return pm

		case msg := <-outCh:
			dest := msg.GetTo()
			if dest == nil {
				for _, P := range parties {
					if P.PartyID().Index == msg.GetFrom().Index {
						continue
					}
					go timedUpdater(P, msg, errCh, pm, P.PartyID().Index)
				}
			} else {
				if dest[0].Index == msg.GetFrom().Index {
					b.Fatalf("party %d tried to send a message to itself (%d)", dest[0].Index, msg.GetFrom().Index)
					return pm
				}
				go timedUpdater(parties[dest[0].Index], msg, errCh, pm, dest[0].Index)
			}

		case <-endCh:
			if atomic.AddInt32(&ended, 1) == int32(n) {
				return pm
			}
		}
	}
}

// timedUpdater wraps SharedPartyUpdater with per-party compute time tracking.
func timedUpdater(party tss.Party, msg tss.Message, errCh chan<- *tss.Error, pm *partyMetrics, pIdx int) {
	start := time.Now()
	test.SharedPartyUpdater(party, msg, errCh)
	pm.add(pIdx, time.Since(start))
}
