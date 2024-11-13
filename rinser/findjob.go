package rinser

import (
	"github.com/google/uuid"
)

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

func (rns *Rinse) JobList(email string) (jobs []*Job) {
	isadmin := rns.IsAdmin(email)
	rns.mu.Lock()
	for _, job := range rns.jobs {
		if isadmin || (!job.Private && job.Email == email) {
			jobs = append(jobs, job)
		}
	}
	rns.mu.Unlock()
	return
}
