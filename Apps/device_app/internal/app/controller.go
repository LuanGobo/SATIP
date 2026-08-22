package app

import "fmt"

type RecipeStep int

const (
	StepFillProd1  RecipeStep = iota
	StepFillProd2
	StepHeat
	StepPressurize
	StepAgitate
	StepTransfer
	StepCIP
	StepDone
)

func (s RecipeStep) String() string {
	switch s {
	case StepFillProd1:
		return "fill_prod1_to_500L"
	case StepFillProd2:
		return "fill_prod2_to_750L"
	case StepHeat:
		return "heat_to_60C"
	case StepPressurize:
		return "pressurize_to_2bar"
	case StepAgitate:
		return "agitate_5min"
	case StepTransfer:
		return "transfer_product"
	case StepCIP:
		return "cip_cleaning"
	case StepDone:
		return "done"
	default:
		return "unknown"
	}
}

// Controller orquestra os comandos (valvulas/motores) com base no estado do processo.
// Ele NÃO simula física; apenas decide ações.
type Controller struct {
	tankID string

	step         RecipeStep
	stepStartSec float64

	// guarda último step para gerar evento "phase_changed"
	lastStep RecipeStep

	// Realimentação de alarmes do tick anterior (setada pelo loop principal).
	// Usada para reagir a falhas do processo com uma ação corretiva.
	// Histerese: entra em correção no alarme HH (175°C) e só sai quando a
	// temperatura cai abaixo do alarme H (160°C) — evita chaveamento rápido
	// da válvula (chattering) perto do limiar HH.
	jacketCorrecting bool
	productOverTemp  bool
	wasCorrecting    bool

	// Comando corrente da válvula de vapor no ciclo de controle da camisa.
	steamCmd bool
}

func NewController(tankID string) *Controller {
	return &Controller{
		tankID:   tankID,
		step:     StepFillProd1,
		lastStep: -1,
	}
}

// SetAlarmFeedback recebe o estado de alarme apurado no tick anterior, para
// que o controlador possa reagir a condições fora de especificação. A
// sobretemperatura do produto (TIT-01) constitui intertravamento de segurança
// e atua em qualquer etapa da receita; a sobretemperatura da camisa (TIT-02)
// atua durante o aquecimento.
func (c *Controller) SetAlarmFeedback(productH, jacketH bool) {
	c.productOverTemp = productH
	c.jacketCorrecting = jacketH
}

type ControlResult struct {
	StepName string
	Events   []string
}

// Apply aplica comandos no Model em função do estado atual (antes do Engine.Step).
func (c *Controller) Apply(m *Model, nowSec float64) (ControlResult, error) {
	tk := m.Tanks[c.tankID]
	if tk == nil {
		return ControlResult{}, fmt.Errorf("controller: tank '%s' not found", c.tankID)
	}

	// Helpers seguros
	openValve := func(id string, open bool) {
		v := m.Valves[id]
		if v == nil {
			return
		}
		if open {
			v.Open()
		} else {
			v.Close()
		}
	}

	startMotorFwd := func(id string, on bool) {
		mm := m.Motors[id]
		if mm == nil {
			return
		}
		if on {
			mm.StartForward()
		} else {
			mm.Stop()
		}
	}

	// IDs do seu YAML
	const (
		xvProd1 = "xv-01"
		xvCIP   = "xv-02"
		xvProd2 = "xv-03"
		xvAir   = "xv-04"
		xvVent  = "xv-05"
		xvSteam = "xv-06"
		xvTrans = "xv-07"

		mAgit = "m-agit-01"
		mt01  = "mt-01"
	)

	// Defaults por tick (garante determinismo)
	openValve(xvProd1, false)
	openValve(xvCIP, false)
	openValve(xvProd2, false)
	openValve(xvAir, false)
	openValve(xvSteam, false)
	openValve(xvVent, false)
	openValve(xvTrans, false)

	startMotorFwd(mAgit, false)
	startMotorFwd(mt01, false)

	var ev []string

	// Evento de mudança de fase (útil para debug e TCC)
	if c.step != c.lastStep {
		ev = append(ev, "phase_changed:"+c.step.String())
		c.lastStep = c.step
	}

	// Sequência da receita farmacêutica
	switch c.step {

	case StepFillProd1:
		openValve(xvProd1, true)
		if tk.VolumeL() >= 500.0 {
			c.step = StepFillProd2
		}

	case StepFillProd2:
		openValve(xvProd2, true)
		if tk.VolumeL() >= 750.0 {
			c.step = StepHeat
		}

	case StepHeat:
		// A válvula de vapor é do tipo abre-fecha, mas o controlador a opera
		// em ciclos curtos para manter a camisa próxima do setpoint, com
		// banda morta estreita. Como o atuador leva alguns segundos para
		// cursar e a camisa tem inércia térmica, o resultado é uma ondulação
		// suave em torno do setpoint, e não um degrau entre extremos.
		const (
			jacketSetpointC = 135.0
			jacketBandC     = 1.5
		)
		jacket := tk.SteamTempC()
		switch {
		case jacket > jacketSetpointC+jacketBandC:
			c.steamCmd = false
		case jacket < jacketSetpointC-jacketBandC:
			c.steamCmd = true
		}
		openValve(xvSteam, c.steamCmd)

		// Abre vent para alívio de pressão se pressão exceder 1.3 bar
		if tk.PressureBar() > 1.3 {
			openValve(xvVent, true)
		}
		if tk.TempC() >= 60.0 {
			c.step = StepPressurize
		}

	case StepPressurize:
		openValve(xvAir, true)
		if tk.PressureBar() >= 2.0 {
			c.step = StepAgitate
			c.stepStartSec = nowSec
		}

	case StepAgitate:
		startMotorFwd(mAgit, true)
		if nowSec-c.stepStartSec >= 300.0 { // 5 min = 300 s
			c.step = StepTransfer
			c.stepStartSec = nowSec
		}

	case StepTransfer:
		openValve(xvTrans, true)
		startMotorFwd(mt01, true)
		// Transfere até tanque quase vazio ou timeout de 10 min
		if tk.VolumeL() < 50.0 || nowSec-c.stepStartSec >= 600.0 {
			c.step = StepCIP
			c.stepStartSec = nowSec
		}

	case StepCIP:
		openValve(xvCIP, true)
		// CIP roda por 5 minutos
		if nowSec-c.stepStartSec >= 300.0 {
			c.step = StepDone
		}

	case StepDone:
		// defaults já deixam tudo off
	}

	// Intertravamento de segurança, sobreposto à decisão normal da etapa: a
	// sobretemperatura do produto força o fechamento da válvula de vapor e a
	// abertura do respiro, em qualquer etapa da receita. Note que o comando
	// de fechamento nem sempre surte efeito — se o atuador estiver obstruído,
	// a passagem residual de vapor persiste e a condição não se resolve, o
	// que é justamente o cenário que só o histórico permite diagnosticar.
	if c.productOverTemp || (c.jacketCorrecting && c.step == StepHeat) {
		openValve(xvSteam, false)
		openValve(xvVent, true)
		if !c.wasCorrecting {
			reason := "jacket_temp_high"
			if c.productOverTemp {
				reason = "product_temp_high"
			}
			ev = append(ev, "corrective_action:steam_closed_"+reason)
			c.wasCorrecting = true
		}
	} else {
		c.wasCorrecting = false
	}

	return ControlResult{
		StepName: c.step.String(),
		Events:   ev,
	}, nil
}
