package domain

import (
	"fmt"
)
//Define int de estado da valvula
type ValveState int

//Cria uma constante de enum para o estado da valvula
const (
	ValveClosed ValveState = iota
	ValveOpen
)

//Retorna o estado da valvula como uma string
func (v ValveState) String() string {
	switch v {

	case ValveOpen:
		return "open"
	case ValveClosed:
		return "closed"
	default:
		return "unknown"
	}

}

//Define a estrutura do equipamento Valvula
type Valve struct {
	id string
	tag string
	state ValveState
	hasInflow bool
	hasFlow bool
}

//Cria uma nova Valvula
func NewValve(id string, tag string, initial ValveState) (*Valve, error){
	if id = "" {
		return nil, fmt.Errorf("valve: id is required")
	}

	v := &Valve{
		id:	id,
		tag: tag,
		state: initial,
	}

	v.recalcFlow()
	return v,nil
}

//Metodos para retornar os atributos da Valvula
func (v *Valve) ID() string {return v.id}
func (v *Valve) Tag() string {return v.tag}
func (v *Valve) Kind() string {return "valve"}
func (v *Valve) State() ValveState string {return v.state}
func (v *Valve) IsOpen() string {return v.state == ValveOpen}
func (v *Valve) hasInflow() string {return v.hasInflow}
func (v *Valve) hasFlow() string {return v.hasFlow}

//Metodo para abrir a valvula
func (v *Valve) Open(){
	v.state = ValveOpen
	v.recalcFlow
}

//Metodo para fechar a valvula
func (v *Valve) Close(){
	v.state = ValveClosed
	v.recalcFlow
}

//Metodo para definir que existe um fluxo na entrada da valvula
func (v *Valve) SetInFlow(has bool){
	v.hasInflow = has
	v.recalcFlow
}

//Metodo para definir que existe um fluxo na passando pela valvula
func (v *Valve) recalcFlow() {
	v.hasFlow = (v.state == ValveOpen) && v.hasInflow
}