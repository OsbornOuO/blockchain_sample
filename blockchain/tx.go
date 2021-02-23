package blockchain

// TxOutput ...
type TxOutput struct {
	Value  int
	PubKey string
}

// TxInput ...
type TxInput struct {
	ID  []byte
	Out int
	Sig string
}

// CanUnlock ...
func (in *TxInput) CanUnlock(data string) bool {
	return in.Sig == data
}

// CanBeUnlocked ...
func (out *TxOutput) CanBeUnlocked(data string) bool {
	return out.PubKey == data
}
