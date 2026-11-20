package rinser

import "testing"

func TestUiJobStatusDiskuse(t *testing.T) {
	got := string(uiJobStatus{&Job{Diskuse: 234079, state: JobFinished}}.JawsGetHTML(nil))
	if want := `<span class="text-success fw-bold">Finished (229kB)</span>`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
