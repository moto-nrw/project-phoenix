package api

func (d seedDate) Compare(other seedDate) int { return d.Time.Compare(other.Time) }
