package eventbus

import (
	"sync"
)

type DataEvent struct {
	Data  interface{}
	Topic string
}

type DataChannel chan DataEvent
type DataChannelSlice []DataChannel

type EventBus struct {
	subscribers map[string]DataChannelSlice
	rm          sync.RWMutex
}

func New() *EventBus {
	return &EventBus{
		subscribers: map[string]DataChannelSlice{},
	}
}

func (eb *EventBus) Subscribe(topic string, ch DataChannel) {
	eb.rm.Lock()
	if prev, found := eb.subscribers[topic]; found {
		eb.subscribers[topic] = append(prev, ch)
	} else {
		eb.subscribers[topic] = append([]DataChannel{}, ch)
	}
	eb.rm.Unlock()
}

func (eb *EventBus) UnSubscribe(topic string, ch DataChannel) {
	eb.rm.Lock()
	if chans, found := eb.subscribers[topic]; found {
		nchans := DataChannelSlice{}
		for _, c := range chans {
			if c == ch {
				continue
			}
			nchans = append(nchans, c)
		}
		eb.subscribers[topic] = nchans
	}
	eb.rm.Unlock()
}

func (eb *EventBus) Publish(topic string, data interface{}) {
	eb.rm.RLock()
	if chans, found := eb.subscribers[topic]; found {
		// this is done because the slices refer to same array even though they are passed by value
		// thus we are creating a new slice with our elements thus preserve locking correctly.
		channels := append(DataChannelSlice{}, chans...)
		go func(data DataEvent, dataChannelSlices DataChannelSlice) {
			for _, ch := range dataChannelSlices {
				ch <- data
			}
		}(DataEvent{Data: data, Topic: topic}, channels)
	}
	eb.rm.RUnlock()
}
