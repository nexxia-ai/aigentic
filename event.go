package aigentic

import "errors"

type Event struct {
	Agent   *Agent
	Message string
	// Run history
	LLMMsg string
}

func (e *Event) Execute() (RunResponse, error) {
	if e.Agent == nil {
		return RunResponse{}, errors.New("agent is nil")
	}
	content, err := e.Agent.generate(e.Message)
	e.Agent.run.Content = content
	return *e.Agent.run, err
}

type EventLoop struct {
	Session *Session
	Agent   *Agent
	next    chan *Event
}

func (e *EventLoop) Next() <-chan *Event {
	return e.next
}
