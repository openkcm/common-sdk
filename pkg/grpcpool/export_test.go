package grpcpool

// IsHealthy verifies if the connection is healthy
func (pcc *PooledClientConn) IsHealthy() bool {
	return pcc.isHealthy()
}

// Capacity returns the capacity
func (p *Pool) Capacity() int {
	if p.IsClosed() {
		return 0
	}
	return cap(p.clientWrappers)
}

// Available returns the number of currently unused clients
func (p *Pool) Available() int {
	if p.IsClosed() {
		return 0
	}
	return len(p.clientWrappers)
}
