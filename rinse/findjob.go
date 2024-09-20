package rinse

import "github.com/google/uuid"

func (rns *Rinse) findJobUuid(u uuid.UUID) *Job {
	rns.mu.Lock()
	defer rns.mu.Unlock()
	for _, job := range rns.jobs {
		if job.UUID == u {
			return job
		}
	}
	return nil
}

func (rns *Rinse) FindJob(s string) *Job {
	if s != "" {
		if u, err := uuid.Parse(s); err == nil {
			return rns.findJobUuid(u)
		}
	}
	return nil
}

func (rns *Rinse) JobList() (jobs []*Job) {
	rns.mu.Lock()
	jobs = append(jobs, rns.jobs...)
	rns.mu.Unlock()
	return
}
