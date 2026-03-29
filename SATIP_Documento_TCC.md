# SATIP - Sistema de Aquisicao e Tratamento de Informacoes de Processo

## 1. Introducao e Visao Geral

O SATIP (Sistema de Aquisicao e Tratamento de Informacoes de Processo) e um projeto desenvolvido como Trabalho de Conclusao de Curso (TCC) em Engenharia de Software, cujo objetivo e demonstrar a construcao completa de um sistema de supervisao industrial moderno, desde a simulacao de uma planta farmaceutica ate a visualizacao de dados em tempo real por meio de um dashboard web.

O sistema simula um processo de producao farmaceutica baseado em um tanque aquecido com agitacao, instrumentacao completa (sensores de temperatura, nivel e pressao), valvulas de controle e bombas de transferencia. Os dados gerados pelo simulador sao armazenados em banco de dados de series temporais e apresentados em uma interface web interativa que permite analise de tendencias, monitoramento de alarmes e visualizacao de diagramas de processo (P&ID).

### Arquitetura em 3 Camadas

```
+---------------------+       +---------------------+       +---------------------+
|   Simulador (Go)    | --->  |   TimescaleDB       | <---  |   Dashboard Web     |
|   Motor de fisica   |       |   (PostgreSQL 16)   |       |   (Next.js + React) |
|   Receita batch     |       |   Hypertable        |       |   Recharts          |
|   Sensores/Alarmes  |       |   Batch COPY        |       |   P&ID SVG          |
+---------------------+       +---------------------+       +---------------------+
        device_app                     Infra                     dashboard
```

---

## 2. Stack Tecnologica

### 2.1 Linguagens e Frameworks

| Camada | Tecnologia | Versao | Finalidade |
|--------|-----------|--------|------------|
| Simulador | Go | 1.23 | Motor de simulacao, controlador de processo, ingestao de dados |
| Banco de Dados | TimescaleDB | PostgreSQL 16 | Armazenamento otimizado de series temporais |
| API Backend | Go + Chi Router | chi/v5 5.0.12 | Endpoints REST para consulta de equipamentos e historico |
| Frontend | Next.js | 14.2.3 | Framework React com App Router para o dashboard |
| UI | React | 18.x | Componentes interativos, gerenciamento de estado |
| Graficos | Recharts | 2.12.7 | Visualizacao de series temporais com graficos de linha |
| Estilizacao | Tailwind CSS | 3.4.1 | Sistema utilitario de CSS para layout responsivo |
| Icones | Lucide React | 0.378.0 | Iconografia dos equipamentos no dashboard |
| Driver DB | pgx/v5 | 5.5.5 | Driver PostgreSQL nativo para Go com suporte a COPY |
| Infraestrutura | Docker Compose | - | Orquestracao de containers (DB, API, Web) |

### 2.2 Justificativa das Escolhas

- **Go** foi escolhido pela performance em operacoes numericas (simulacao de fisica), concorrencia nativa e compilacao em binario unico (ideal para containers Docker).
- **TimescaleDB** e uma extensao do PostgreSQL especializada em series temporais, oferecendo particao automatica (hypertables), compressao de dados antigos e funcoes de agregacao temporal nativas - ideal para dados de processo industrial.
- **Next.js com React** proporciona Server-Side Rendering, roteamento baseado em arquivos e excelente experiencia de desenvolvimento com TypeScript.
- **Recharts** e construido sobre componentes React declarativos, facilitando a criacao de graficos interativos sem dependencias pesadas como D3 puro.
- **Docker Compose** permite que toda a infraestrutura (banco, API, frontend) seja levantada com um unico comando, garantindo reproducibilidade.

---

## 3. Simulador de Processo Industrial

O simulador e o nucleo do sistema, implementado inteiramente em Go. Ele modela uma planta farmaceutica com fisica simplificada mas realista, operando em tempo discreto.

### 3.1 Motor de Simulacao (Engine)

O motor de simulacao opera com timestep fixo (dt = 1 segundo por padrao) e executa a seguinte sequencia a cada tick:

1. **Reset de entradas**: limpa todas as variaveis de inflow, vapor, ar, ventilacao e agitacao
2. **Processamento de conexoes**: avalia cada conexao da planta (tubulacoes, jaquetas de vapor, ar limpo, etc.) e acumula fluxos nos tanques de destino
3. **Avanco da fisica do tanque**: calcula balanco de massa, transferencia de calor e pressao
4. **Avaliacao de sensores**: atualiza leituras com deadband e verifica condicoes de alarme
5. **Geracao de snapshots**: captura o estado completo de todos os equipamentos
6. **Retorno do StepResult**: estrutura contendo estados de tanques, sensores, valvulas, motores e eventos

```
Loop Principal (20.000 ticks = ~5.5 horas simuladas):
  Para cada tick i:
    1. Controller.Apply() → decide quais valvulas abrir, motores ligar
    2. Engine.Step()      → avanca fisica, gera StepResult
    3. Log eventos        → grava mudancas de estado (on-change)
    4. Batch de metricas  → acumula dados no buffer
    5. A cada 100 ticks   → FlushBatch() envia dados ao TimescaleDB
```

### 3.2 Modelos de Equipamento

#### 3.2.1 Tanque (Tank)

O tanque e o equipamento central, modelando um reator farmaceutico com as seguintes variaveis de estado:

- **Volume** (L): balanco de massa entre entrada e saida
- **Temperatura** (C): modelo de transferencia de calor com jaqueta de vapor
- **Pressao** (bar): pressao do headspace (espaco acima do liquido) com ar limpo e alívio por vent

**Modelo de Fisica:**

```
Balanco de Massa:
  volume += (inflowLpm / 60) * dt
  volume = clamp(volume, 0, capacidade)

Transferencia de Calor (Jaqueta de Vapor):
  steamTemp += tau * (supplyTemp - steamTemp) * dt     [tau = 0.02 aquecendo, 0.005 resfriando]
  tempProduto += (UA_eff / (massa * Cp)) * (steamTemp - tempProduto) * dt
  UA_eff = 900 W/K (x1.25 com agitacao)
  Cp = 4180 J/(kg*K)

Pressao do Headspace:
  pressGain = (airFlowNlpm / headspaceL) * 0.002
  pressRelief = (pressao - 1.0) * 1.5    [se vent aberto]
  pressao += (pressGain - pressRelief) * dt
```

**Configuracao (plant.yaml):**
```yaml
tanks:
  - id: tk-01
    tag: TK-01
    capacity_l: 1000
    initial_volume_l: 200
    initial_temp_c: 25
```

#### 3.2.2 Valvulas (Valve)

Modelam valvulas on/off (abertas ou fechadas) com logica de fluxo:

- **Maquina de estados**: `Closed <-> Open`
- **Logica de fluxo**: `flowActive = isOpen AND inflowPresent`
- **Verificacao de caminho**: o motor verifica se todas as valvulas e motores no caminho estao ativos antes de permitir fluxo

**Tipos de servico configurados:**
- `product` (XV-01, XV-03): entrada de materia-prima
- `cip` (XV-02): limpeza CIP (Clean-in-Place)
- `clean_air` (XV-04): pressurizacao com ar limpo
- `vent` (XV-05): alivio de pressao
- `steam` (XV-06): jaqueta de vapor
- `transfer` (XV-07): saida de produto via bomba

#### 3.2.3 Motores (Motor)

Modelam agitadores e bombas com 3 estados:

- **Maquina de estados**: `Stopped <-> RunningForward <-> RunningReverse`
- **Logica de atividade**: `isActive = (running) AND hasLoad`
- **RPM**: 150.5 quando ativo, 0 quando parado
- **Motores configurados**:
  - `AG-01` (agitador do tanque, com reverso habilitado)
  - `MT-01` (bomba de transferencia)

#### 3.2.4 Sensores Analogicos (AnalogSensor)

Os sensores sao o elo entre a fisica do tanque e o sistema de supervisao:

- **Deadband**: filtragem de ruido — so atualiza o valor lido se a diferenca ultrapassar o limiar configurado
- **4 niveis de alarme**: LL (Low-Low), L (Low), H (High), HH (High-High)
- **Deteccao de borda**: alarmes disparam apenas na transicao 0→1, evitando eventos repetidos

**Sensores configurados:**

| ID | Tag | Tipo | Fonte | Deadband | Alarmes |
|----|-----|------|-------|----------|---------|
| tit-01 | TIT-01 | Temperatura do produto | tank.tk-01.temp | 0.5 C | LL=10, L=20, H=80, HH=90 |
| tit-02 | TIT-02 | Temperatura da jaqueta | tank.tk-01.steam_temp | 1.0 C | H=130, HH=140 |
| lit-01 | LIT-01 | Nivel do tanque (%) | tank.tk-01.level_pct | 0.2% | LL=5, L=10, H=90, HH=95 |
| pit-01 | PIT-01 | Pressao do headspace | tank.tk-01.pressure_bar | 0.05 bar | H=2.5, HH=3.0 |

### 3.3 Conexoes da Planta

O motor de simulacao processa 6 tipos de conexao:

| Tipo | Descricao | Exemplo |
|------|-----------|---------|
| `product_inlet` | Entrada de materia-prima com vazao e temperatura | Produto 1: 50 L/min a 25C via XV-01 |
| `cip_inlet` | Entrada de solucao de limpeza | CIP: 80 L/min a 60C via XV-02 |
| `steam_jacket` | Aquecimento por jaqueta de vapor | Vapor a 140C via XV-06 |
| `clean_air_inlet` | Pressurizacao com ar filtrado | 100 NL/min via XV-04 |
| `vent_outlet` | Alivio de pressao do headspace | Via XV-05 |
| `transfer_outlet` | Saida de produto via bomba | 40 L/min via XV-07 + MT-01 |
| `agitation_link` | Acoplamento mecanico motor-tanque | AG-01 requer volume > 10L |

### 3.4 Receita de Producao (Batch)

O controlador implementa uma receita de producao farmaceutica em 8 etapas sequenciais:

```
 Etapa 1        Etapa 2        Etapa 3         Etapa 4
 FILL PROD 1 -> FILL PROD 2 -> AQUECIMENTO  -> PRESSURIZACAO
 XV-01 aberta   XV-03 aberta   XV-06 (vapor)   XV-04 (ar)
 ate 500L       ate 750L       XV-05 (vent)    ate 2.0 bar
                               ate 60C

 Etapa 5        Etapa 6        Etapa 7         Etapa 8
 AGITACAO    -> TRANSFERENCIA-> CIP          -> CONCLUIDO
 AG-01 ligado   XV-07 + MT-01  XV-02 aberta    Tudo desligado
 300 segundos   ate vol < 50L  300 segundos
```

**Detalhamento das etapas:**

1. **FillProd1**: Abertura da valvula XV-01 para entrada do Produto 1 (25C, 50 L/min). Transicao quando volume >= 500L.
2. **FillProd2**: Abertura da valvula XV-03 para entrada do Produto 2 (30C, 30 L/min). Transicao quando volume >= 750L.
3. **Heat (Aquecimento)**: Abertura da valvula XV-06 para circulacao de vapor na jaqueta (140C). Valvula XV-05 (vent) abre automaticamente se pressao > 1.3 bar. Transicao quando temperatura >= 60C.
4. **Pressurize (Pressurizacao)**: Abertura da valvula XV-04 para injecao de ar limpo. Transicao quando pressao >= 2.0 bar.
5. **Agitate (Agitacao)**: Acionamento do agitador AG-01 em sentido direto (forward). Melhora a homogeneidade termica (fator alpha de 0.05 para 0.15). Duracao fixa de 300 segundos.
6. **Transfer (Transferencia)**: Abertura da valvula XV-07 e acionamento da bomba MT-01 para transferencia do produto. Transicao quando volume < 50L ou timeout de 600 segundos.
7. **CIP (Clean-in-Place)**: Abertura da valvula XV-02 para entrada de solucao de limpeza (80 L/min a 60C). Duracao fixa de 300 segundos.
8. **Done (Concluido)**: Todas as valvulas fechadas e motores desligados. Fim do batch.

---

## 4. Camada de Armazenamento (TimescaleDB)

### 4.1 Escolha do TimescaleDB

O TimescaleDB e uma extensao do PostgreSQL projetada especificamente para dados de series temporais. Diferente de bancos NoSQL como InfluxDB, ele mantem compatibilidade total com SQL, permitindo JOINs com tabelas relacionais (equipamentos, usuarios) enquanto oferece:

- **Hypertables**: tabelas particionadas automaticamente por tempo
- **Compressao nativa**: reduz armazenamento em ate 95% para dados antigos
- **Funcoes temporais**: `time_bucket()`, `first()`, `last()` para agregacoes
- **Continuous Aggregates**: views materializadas que se atualizam automaticamente

### 4.2 Schema do Banco

```sql
-- Tabela de tipos de equipamento
CREATE TABLE equipment_type (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE     -- 'tank', 'valve', 'motor', 'sensor'
);

-- Tabela de equipamentos com metadados flexiveis
CREATE TABLE equipment (
    id TEXT PRIMARY KEY,           -- 'tk-01', 'xv-01', 'tit-01'
    tag TEXT NOT NULL,             -- 'TK-01', 'XV-01', 'TIT-01'
    type_id INTEGER REFERENCES equipment_type(id),
    metadata JSONB                 -- capacidade, servico, deadband, etc.
);

-- Tabela de usuarios para controle de acesso
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT DEFAULT 'operator'   -- 'admin' ou 'operator'
);

-- Hypertable principal de series temporais
CREATE TABLE history (
    time TIMESTAMPTZ NOT NULL,
    equipment_id TEXT NOT NULL REFERENCES equipment(id),
    metric TEXT NOT NULL,          -- 'temp_c', 'volume_l', 'is_open', etc.
    val_f DOUBLE PRECISION,       -- valores numericos
    val_t TEXT                     -- valores textuais (estado, fase)
);

SELECT create_hypertable('history', 'time', if_not_exists => TRUE);

CREATE INDEX idx_history_equipment_metric
    ON history (equipment_id, metric, time DESC);
```

**Decisoes de projeto:**
- **JSONB na coluna metadata**: permite armazenar configuracoes especificas por tipo de equipamento (capacidade para tanques, deadband para sensores, servico para valvulas) sem necessidade de tabelas auxiliares.
- **Duas colunas de valor (val_f, val_t)**: suporta tanto metricas numericas (temperatura, pressao) quanto textuais (nome da fase, estado do motor) na mesma tabela, evitando duplicacao de schema.
- **Indice composto (equipment_id, metric, time DESC)**: otimizado para o padrao de consulta mais frequente: "qual o historico da metrica X do equipamento Y?"

### 4.3 Estrategias de Ingestao de Dados

O sistema implementa 3 padroes de ingestao, cada um otimizado para um tipo de dado:

#### Padrao 1: LogOnChange (Eventos Esparsos)
```go
func LogOnChange(ctx, time, equipmentID, metric, value)
```
- **Uso**: transicoes de estado (valvula abriu/fechou, fase da receita mudou, alarme ativou)
- **Logica**: compara com o ultimo valor armazenado em cache; so insere se diferente
- **Vantagem**: economia massiva de armazenamento para sinais binarios que mudam raramente

#### Padrao 2: LogAlways (Metricas Continuas)
```go
func LogAlways(ctx, time, equipmentID, metric, value)
```
- **Uso**: leituras de sensores com deadband (temperatura, pressao)
- **Logica**: insere incondicionalmente a cada chamada
- **Vantagem**: garante granularidade temporal completa

#### Padrao 3: Batch COPY (Alta Performance)
```go
func AddToBatch(time, equipmentID, metric, value)
func FlushBatch(ctx)
```
- **Uso**: todas as metricas periodicas (estados de tanque, valvulas, motores, sensores)
- **Logica**: acumula linhas em buffer de memoria e envia via PostgreSQL COPY FROM (protocolo binario nativo)
- **Performance**: ~100x mais rapido que INSERTs individuais
- **Frequencia**: flush a cada 100 ticks da simulacao

### 4.4 Volume de Dados

Uma simulacao completa (20.000 ticks) gera aproximadamente:

| Categoria | Metricas por tick | Total estimado |
|-----------|------------------|----------------|
| Tanque (4 metricas) | 4 | 80.000 registros |
| Sensores (4 sensores x valor) | 4 | 80.000 registros |
| Alarmes (on-change) | variavel | ~200 registros |
| Valvulas (7 x is_open) | 7 | 140.000 registros |
| Motores (2 x active + rpm) | 4 | 80.000 registros |
| Controlador (fase) | on-change | ~8 registros |
| **Total** | | **~380.000+ registros** |

---

## 5. Dashboard Web

### 5.1 Arquitetura do Frontend

O dashboard e construido com Next.js 14 usando o padrao App Router, com dois modulos principais:

#### Pagina de Analise de Processo (Dashboard Principal)
- **Grafico interativo multi-serie**: permite visualizar multiplos equipamentos e metricas simultaneamente no mesmo grafico (ex: temperatura do produto + pressao + nivel no mesmo eixo temporal)
- **Sidebar de equipamentos**: lista todos os equipamentos agrupados por tipo (tanques, valvulas, motores, sensores), com icones distintos por categoria
- **Selecao multi-equipamento**: suporta clique simples, Ctrl+Click para multi-selecao, e drag-and-drop
- **Legenda clicavel**: clicar em uma serie na legenda foca o eixo Y naquela metrica especifica, com ranges pre-definidos por tipo
- **Escalas inteligentes**: ranges automaticos por tipo de metrica (temperatura: 0-150C, pressao: 0-3 bar, nivel: 0-100%, volume: 0-1100L)
- **Tema dark/light**: alternancia entre modos escuro e claro com variaveis CSS customizadas

#### Pagina de Processos (Diagrama P&ID)
- **Diagrama P&ID em SVG**: representacao grafica completa da planta com tanque, valvulas, sensores, motores, tubulacoes e fluxos
- **Informacoes de processo**: vazoes nominais, temperaturas de alimentacao e especificacoes de equipamentos diretamente no diagrama
- **Codificacao por cores**: cada tipo de equipamento (tubulacoes, valvulas, tanques, sensores) possui cor distinta para facilitar identificacao visual

### 5.2 API REST (Backend Go)

| Endpoint | Metodo | Descricao |
|----------|--------|-----------|
| `/api/equipment` | GET | Lista todos os equipamentos com tipo e metadados JSONB |
| `/api/history/{id}` | GET | Retorna historico de series temporais. Query param `?metric=` filtra por metrica |
| `/api/simulation/start` | POST | Inicia a simulacao (spawna o binario do simulador como subprocesso) |
| `/api/simulation/data` | DELETE | Limpa todos os registros historicos |
| `/api/health` | GET | Health check |

### 5.3 Funcionalidades do Dashboard

| Feature | Descricao | Tecnologia |
|---------|-----------|------------|
| Grafico de linhas multi-serie | Multiplas metricas de multiplos equipamentos no mesmo grafico temporal | Recharts LineChart |
| Tooltip interativo | Hover mostra valores formatados de todas as series no ponto temporal | Recharts Tooltip |
| Legenda com foco de eixo | Click na legenda ajusta o eixo Y para a escala da metrica selecionada | React state + Recharts YAxis |
| Visualizacao de alarmes | Series de alarme renderizadas como step functions com tracos mais grossos | Recharts Line (stepAfter) |
| Selecao por drag-drop | Arrastar equipamento da sidebar para o grafico | HTML5 Drag and Drop API |
| Diagrama P&ID | Visualizacao da planta completa com fluxos, equipamentos e instrumentacao | SVG inline com React |
| Controle de simulacao | Botao para iniciar nova simulacao diretamente do dashboard | Fetch API + POST |
| Tema dark/light | Alternancia de tema preservada na sessao | CSS Custom Properties |

---

## 6. Infraestrutura (Docker Compose)

O projeto e totalmente conteinerizado com 3 servicos:

```yaml
services:
  timescaledb:        # Banco de dados TimescaleDB (PostgreSQL 16)
    image: timescale/timescaledb:latest-pg16
    ports: [5432:5432]
    volumes:
      - init.sql:/docker-entrypoint-initdb.d/init.sql   # Schema automatico
      - timescale_data:/var/lib/postgresql/data           # Persistencia

  dashboard_api:      # API REST em Go
    depends_on: [timescaledb]
    ports: [8080:8080]

  dashboard_web:      # Frontend Next.js
    depends_on: [dashboard_api]
    ports: [3000:3000]
```

**Para executar o sistema completo:**
```bash
cd Infra
docker compose up --build
```

---

## 7. Utilizacao Estrategica dos Dados Coletados

Os dados de serie temporal gerados pelo SATIP representam o tipo de informacao que, em ambientes industriais reais, e a base para decisoes estrategicas em multiplas dimensoes:

### 7.1 Analise de Tendencias e Deteccao de Anomalias

Os dados historicos de temperatura, pressao e nivel permitem identificar padroes de comportamento normal do processo e detectar desvios antes que se tornem problemas criticos. Por exemplo:
- **Deriva termica**: se a jaqueta de vapor leva progressivamente mais tempo para aquecer o tanque entre batches, pode indicar incrustacao na superficie de troca termica
- **Anomalia de pressao**: picos inesperados de pressao durante a fase de pressurizacao podem indicar obstrucao no sistema de vent
- **Variacao de nivel**: enchimento mais lento que o nominal pode sinalizar desgaste em valvulas ou bombas

### 7.2 Otimizacao de Parametros de Processo

Com dados historicos suficientes, e possivel:
- Determinar a **temperatura ideal de aquecimento** que minimiza o tempo de batch sem comprometer a qualidade
- Otimizar a **sequencia de abertura de valvulas** para reduzir tempo de transicao entre fases
- Ajustar **thresholds de alarme** baseado no comportamento real (reduzindo alarmes espurios sem comprometer seguranca)

### 7.3 Rastreabilidade e Compliance Regulatorio

Na industria farmaceutica, regulamentacoes como GMP (Good Manufacturing Practices) e normas da ANVISA exigem:
- **Registro completo de cada batch**: temperaturas, pressoes e tempos em cada etapa
- **Trilha de auditoria**: quem operou, quando e quais parametros foram utilizados
- **Evidencia de que o processo permaneceu dentro das especificacoes** durante toda a producao

O SATIP, ao armazenar todas as variaveis de processo com timestamp preciso, demonstra a infraestrutura necessaria para atender essas exigencias.

### 7.4 Reducao de Perdas e Retrabalho

A analise pos-batch permite identificar:
- Batches que ficaram fora de especificacao (temperatura abaixo do minimo, tempo insuficiente de agitacao)
- Correlacao entre parametros de processo e qualidade do produto final
- Causas raiz de desvios (ex: falha no vapor causou subaquecimento, resultando em batch fora de especificacao)

### 7.5 Tomada de Decisao Baseada em Dados

Ao invez de decisoes baseadas em experiencia individual ou "feeling", os dados permitem:
- **Benchmarking entre turnos**: comparar desempenho de diferentes equipes operacionais
- **Planejamento de manutencao**: identificar equipamentos que estao degradando antes da falha
- **Capacity planning**: estimar quantos batches podem ser produzidos com base nos tempos reais de cada etapa

---

## 8. Extensoes Futuras

### 8.1 Calculo Automatizado de OEE (Overall Equipment Effectiveness)

O OEE e o indicador mais utilizado na industria para medir a eficiencia de equipamentos produtivos. Ele e composto por 3 fatores:

```
OEE = Disponibilidade x Performance x Qualidade

Disponibilidade = Tempo Produtivo / Tempo Planejado
Performance     = Producao Real / Producao Teorica
Qualidade       = Produtos Bons / Producao Total
```

**Como o SATIP viabiliza o calculo automatico:**

| Fator OEE | Dados ja disponveis no SATIP | Extensao necessaria |
|-----------|------------------------------|---------------------|
| **Disponibilidade** | Timestamps de inicio/fim de cada batch, transicoes de estado de equipamentos, alarmes | Classificar tempos em: produtivo, setup, manutencao, parada nao planejada |
| **Performance** | Duracao real de cada etapa da receita vs. duracao nominal | Definir tempos nominais por etapa e calcular desvios |
| **Qualidade** | Parametros de processo (temperatura, tempo, pressao) de cada batch | Definir especificacoes de produto e classificar batches como conformes/nao conformes |

**Implementacao sugerida:**
1. Criar tabela `batch_runs` (id, start_time, end_time, recipe, status)
2. Criar tabela `oee_metrics` (batch_id, availability, performance, quality, oee)
3. Implementar funcao que analisa o historico de cada batch e calcula os 3 fatores
4. Dashboard com grafico de OEE ao longo do tempo, com drill-down por fator

### 8.2 Integracao com Sistemas ERP

Sistemas ERP como SAP, TOTVS Protheus ou Oracle gerenciam ordens de producao, estoque e custos. A integracao permitiria:

**Fluxo de dados ERP → SATIP:**
- Receber ordens de producao (receita, quantidade, prazo)
- Parametrizar o simulador automaticamente com base na ordem
- Reservar materiais no estoque antes de iniciar o batch

**Fluxo de dados SATIP → ERP:**
- Reportar consumo real de materia-prima (volume dosado por valvula)
- Reportar tempo de producao por batch (para custeio por absorpcao)
- Atualizar status da ordem de producao em tempo real
- Gerar apontamentos de producao automaticos

**Protocolos de integracao comuns:**
- REST API (ja implementado no SATIP)
- OPC-UA (padrao da industria 4.0 para comunicacao entre sistemas)
- MQTT (mensageria leve para IoT industrial)
- Webhooks para notificacoes assincronas

### 8.3 Integracao com MES (Manufacturing Execution System)

Um MES preenche a lacuna entre o ERP (planejamento) e o chao de fabrica (execucao):

- **Rastreabilidade de lote**: associar cada batch a materia-prima utilizada, parametros de processo e operador responsavel
- **Genealogia de produto**: rastrear toda a cadeia de producao de um lote ate as materias-primas de origem
- **Instrucoes de trabalho eletronicas**: guiar o operador passo a passo pela receita
- **Controle estatistico de processo (CEP)**: cartas de controle automaticas com dados do SATIP

### 8.4 Digital Twin (Gemeo Digital)

O simulador do SATIP ja e, essencialmente, um gemeo digital offline. A extensao para Digital Twin em tempo real envolveria:

- **Conexao com sensores reais**: substituir o motor de fisica por leituras de instrumentacao real via OPC-UA ou MQTT
- **Simulacao preditiva**: usar o modelo matematico para prever o comportamento futuro do processo com base no estado atual
- **Cenarios what-if**: simular o impacto de mudancas de parametros antes de aplica-las na planta real
- **Deteccao de anomalias em tempo real**: comparar dados reais com o modelo teorico e alertar quando divergirem

### 8.5 Machine Learning e Inteligencia Artificial

Os dados de serie temporal coletados pelo SATIP sao a base para modelos de ML:

- **Manutencao preditiva**: modelos de regressao que estimam tempo ate falha baseado em tendencias de degradacao (ex: tempo crescente de aquecimento)
- **Classificacao de qualidade**: modelos que predizem se um batch estara dentro da especificacao com base nos parametros das primeiras etapas
- **Otimizacao de receita**: algoritmos de otimizacao (ex: Bayesian Optimization) para encontrar os parametros ideais de temperatura, pressao e tempo de agitacao
- **Deteccao de anomalias**: autoencoders ou Isolation Forest para detectar padroes anormais nos dados de processo

### 8.6 Expansao da Planta Simulada

O design modular do SATIP (YAML declarativo + engine generica) permite expandir a planta sem alterar o motor de simulacao:

- Adicionar multiplos tanques em paralelo (producao simultânea)
- Simular linhas de envase com controle de velocidade
- Modelar sistemas de utilidades (agua purificada, vapor industrial, ar comprimido)
- Simular falhas de equipamento (valvula travada, sensor com drift, perda de vapor)

---

## 9. Resumo das Tecnologias e Componentes

| Componente | Tecnologia | Proposito |
|------------|-----------|-----------|
| Motor de simulacao | Go 1.23 | Simulacao deterministica de processo industrial com timestep fixo |
| Configuracao da planta | YAML | Descricao declarativa de equipamentos, conexoes e parametros |
| Banco de series temporais | TimescaleDB (PG 16) | Armazenamento otimizado com hypertables e compressao |
| Driver de banco | pgx/v5 | Acesso nativo a PostgreSQL com suporte a COPY batch |
| Ingestao de dados | Batch COPY + on-change | Alta performance com ~100x menos round-trips que INSERT |
| API REST | Go + Chi Router | Endpoints para consulta de equipamentos e historico |
| Frontend | Next.js 14 + React 18 | Dashboard SPA com App Router e TypeScript |
| Graficos | Recharts 2.12 | Visualizacao interativa de series temporais |
| Estilizacao | Tailwind CSS 3.4 | Design responsivo com classes utilitarias |
| Icones | Lucide React | Iconografia dos equipamentos |
| Infraestrutura | Docker Compose | Orquestracao de 3 containers (DB, API, Web) |
| Controle de versao | Git | Versionamento do codigo-fonte |

---

## 10. Conclusao

O SATIP demonstra a viabilidade de construir um sistema completo de aquisicao e tratamento de dados industriais utilizando tecnologias modernas de codigo aberto. O projeto abrange desde a modelagem fisica de equipamentos farmaceuticos ate a visualizacao interativa de dados, passando por uma camada de armazenamento otimizada para series temporais.

A arquitetura em 3 camadas desacopladas (simulador, banco, dashboard) permite evolucao independente de cada componente, e o uso de Docker Compose garante reproducibilidade e facilidade de implantacao. O design declarativo da planta (YAML) e o motor de simulacao generico permitem expandir o sistema para cenarios mais complexos sem necessidade de reescrever a logica de negocio.

Os dados coletados pelo sistema formam a base para aplicacoes de alto valor estrategico como calculo automatizado de OEE, integracao com ERP/MES, gemeos digitais e modelos de Machine Learning — demonstrando que um projeto de TCC pode servir como prova de conceito para solucoes reais da Industria 4.0.
