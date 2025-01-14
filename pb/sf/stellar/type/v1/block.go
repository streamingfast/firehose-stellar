package pbstellar

import (
	fmt "fmt"

	xdrTypes "github.com/stellar/go/xdr"
)

func FromXdrContactEventType(x xdrTypes.ContractEventType) ContractEvent_Type {
	switch x {
	case xdrTypes.ContractEventTypeSystem:
		return ContractEvent_SYSTEM
	case xdrTypes.ContractEventTypeContract:
		return ContractEvent_CONTRACT
	case xdrTypes.ContractEventTypeDiagnostic:
		return ContractEvent_DIAGNOSTIC
	}

	panic(fmt.Sprintf("Unknown xdr type %T", x))
}
