// @ts-nocheck
"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import {
  Thermometer,
  Droplets,
  ChevronRight,
  ChevronLeft,
  Box,
  Zap,
  FlaskConical,
  X,
} from "lucide-react";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  ReferenceArea,
} from "recharts";

const API_URL = "http://localhost:8080/api";

const COLORS = [
  "#6366f1", "#10b981", "#f59e0b", "#ef4444", "#8b5cf6", "#ec4899",
  "#14b8a6", "#f97316", "#06b6d4", "#84cc16", "#e879f9", "#fb923c"
];

const METRIC_LABELS: Record<string, string> = {
  volume_l: "Volume (L)",
  level_pct: "Nível (%)",
  temp_c: "Temperatura (°C)",
  pressure_bar: "Pressão (bar)",
  rpm: "Rotação (rpm)",
  is_open: "Aberta",
  active: "Ativo",
  position: "Abertura (0–1)",
  value: "Valor",
  alarm_ll: "Alarme LL",
  alarm_l: "Alarme L",
  alarm_h: "Alarme H",
  alarm_hh: "Alarme HH",
};

// Rótulo da grandeza medida por cada transmissor, conforme o campo `kind`
// gravado nos metadados do equipamento. Sem isso a legenda de um sensor
// mostraria apenas "Valor", sem dizer se a curva é temperatura, nível ou
// pressão — justamente o que a figura precisa deixar claro.
const SENSOR_KIND_LABELS: Record<string, string> = {
  temp_product: "Temperatura do produto (°C)",
  temp_steam: "Temperatura da camisa (°C)",
  level_pct: "Nível (%)",
  pressure: "Pressão (bar)",
};

// Nomes das categorias na barra lateral. O agrupamento vem do tipo do
// equipamento, que é gravado em inglês no banco.
const CATEGORY_LABELS: Record<string, string> = {
  tank: "Tanques",
  valve: "Válvulas",
  motor: "Motores",
  sensor: "Sensores",
  controller: "Controlador",
  outros: "Outros",
};

// Metrics to hide from chart
const HIDDEN_METRICS = new Set(["has_flow"]);

// Predefined Y-axis ranges per metric type
const METRIC_RANGES: Record<string, [number, number]> = {
  volume_l: [0, 1100],
  level_pct: [0, 100],
  temp_c: [0, 150],
  pressure_bar: [0, 3],
  rpm: [0, 200],
  is_open: [0, 1.2],
  active: [0, 1.2],
  value: [0, 150], // generic sensor value
};

export default function DashboardPage() {
  const [equipments, setEquipments] = useState([]);
  const [selectedEqIds, setSelectedEqIds] = useState([]);
  const [allHistory, setAllHistory] = useState({});

  // UI State
  const [eqSidebarOpen, setEqSidebarOpen] = useState(true);
  const [collapsedCats, setCollapsedCats] = useState({});
  const [activeYAxisKey, setActiveYAxisKey] = useState<string | null>(null);
  const [hoveredLegend, setHoveredLegend] = useState<string | null>(null);

  // Selecao de janela de tempo por arrasto sobre o grafico
  const [refAreaLeft, setRefAreaLeft] = useState<number | null>(null);
  const [refAreaRight, setRefAreaRight] = useState<number | null>(null);
  const [isSelecting, setIsSelecting] = useState(false);
  const [zoomDomain, setZoomDomain] = useState<[number, number] | null>(null);

  useEffect(() => {
    fetch(`${API_URL}/equipment`)
      .then(res => res.json())
      .then(data => {
        setEquipments(data);
        if (data.length > 0) setSelectedEqIds([data[0].id]);
      });
  }, []);

  useEffect(() => {
    selectedEqIds.forEach(id => {
      if (!allHistory[id]) {
        fetch(`${API_URL}/history/${id}`)
          .then(res => res.json())
          .then(data => {
            setAllHistory(prev => ({ ...prev, [id]: data }));
          });
      }
    });
  }, [selectedEqIds]);

  const toggleEq = (id, isMulti = false) => {
    if (isMulti) {
      setSelectedEqIds(prev =>
        prev.includes(id) ? prev.filter(i => i !== id) : [...prev, id]
      );
    } else {
      setSelectedEqIds([id]);
    }
  };

  const toggleCategory = (cat) => {
    setCollapsedCats(prev => ({ ...prev, [cat]: !prev[cat] }));
  };

  const onDragStart = (e, id) => {
    e.dataTransfer.setData("eqId", id);
  };

  const onDrop = (e) => {
    e.preventDefault();
    const id = e.dataTransfer.getData("eqId");
    if (id) toggleEq(id, true);
  };

  const onDragOver = (e) => {
    e.preventDefault();
  };

  const groupedEquipments = useMemo(() => {
    return (equipments as any[]).reduce((acc: any, eq: any) => {
      const type = eq.type || 'outros';
      if (!acc[type]) acc[type] = [];
      acc[type].push(eq);
      return acc;
    }, {});
  }, [equipments]);

  // Merge history data for Recharts (multi-metric, filtering hidden metrics)
  const { chartData, seriesKeys } = useMemo(() => {
    const timeMap: Record<number, any> = {};
    const keys = new Set<string>();

    (selectedEqIds as any[]).forEach(id => {
      const data = (allHistory as any)[id] || [];
      data.forEach((point: any) => {
        if (point.value === null || point.value === undefined) return;
        if (typeof point.value === 'string') return;

        const metric = point.metric || 'value';
        if (HIDDEN_METRICS.has(metric)) return;

        const d = new Date(point.time);
        const ts = d.getTime(); // chave numérica (evita colisão e ordenação lexicográfica)
        const seriesKey = `${id}::${metric}`;
        keys.add(seriesKey);

        if (!timeMap[ts]) timeMap[ts] = { ts, time: d.toLocaleTimeString() };
        timeMap[ts][seriesKey] = point.value;
      });
    });

    return {
      chartData: Object.values(timeMap).sort((a: any, b: any) => a.ts - b.ts),
      seriesKeys: Array.from(keys),
    };
  }, [allHistory, selectedEqIds]);

  // Handlers da selecao por arrasto: pressiona, arrasta, solta e filtra.
  const handleMouseDown = useCallback((e: any) => {
    if (!e || e.activeLabel === undefined || e.activeLabel === null) return;
    setRefAreaLeft(Number(e.activeLabel));
    setRefAreaRight(null);
    setIsSelecting(true);
  }, []);

  const handleMouseMove = useCallback((e: any) => {
    if (!isSelecting) return;
    if (!e || e.activeLabel === undefined || e.activeLabel === null) return;
    setRefAreaRight(Number(e.activeLabel));
  }, [isSelecting]);

  const handleMouseUp = useCallback(() => {
    setIsSelecting(false);
    if (refAreaLeft === null || refAreaRight === null || refAreaLeft === refAreaRight) {
      setRefAreaLeft(null);
      setRefAreaRight(null);
      return;
    }
    const left = Math.min(refAreaLeft, refAreaRight);
    const right = Math.max(refAreaLeft, refAreaRight);
    setZoomDomain([left, right]);
    setRefAreaLeft(null);
    setRefAreaRight(null);
  }, [refAreaLeft, refAreaRight]);

  const resetZoom = useCallback(() => {
    setZoomDomain(null);
    setRefAreaLeft(null);
    setRefAreaRight(null);
    setIsSelecting(false);
  }, []);

  // Linhas visiveis na janela atual (usadas para escalar o eixo Y)
  const visibleData = useMemo(() => {
    if (!zoomDomain) return chartData;
    return (chartData as any[]).filter(
      (row: any) => row.ts >= zoomDomain[0] && row.ts <= zoomDomain[1]
    );
  }, [chartData, zoomDomain]);

  // Compute Y-axis domain based on active legend selection
  const yAxisDomain = useMemo(() => {
    if (activeYAxisKey) {
      const [, metric] = activeYAxisKey.split("::");
      const range = METRIC_RANGES[metric];
      if (range) return range;
    }
    // Auto: computa a partir dos dados visiveis na janela atual
    let min = Infinity;
    let max = -Infinity;
    seriesKeys.forEach(key => {
      visibleData.forEach(row => {
        const v = row[key];
        if (v !== undefined && v !== null) {
          if (v < min) min = v;
          if (v > max) max = v;
        }
      });
    });
    if (min === Infinity) return [0, 100];
    const padding = (max - min) * 0.1 || 1;
    return [Math.max(0, Math.floor(min - padding)), Math.ceil(max + padding)];
  }, [activeYAxisKey, seriesKeys, visibleData]);

  const handleLegendClick = useCallback((key: string) => {
    setActiveYAxisKey(prev => prev === key ? null : key);
  }, []);

  const getSeriesLabel = useCallback((key: string) => {
    const [eqId, metric] = key.split("::");
    const eq = (equipments as any[]).find(e => e.id === eqId);

    let rotulo = METRIC_LABELS[metric] || metric;

    // Para a leitura principal de um transmissor, "Valor" nao informa a
    // grandeza. Substitui pelo `kind` dos metadados quando disponivel;
    // se a API nao devolver o campo, mantem o rotulo generico.
    if (metric === "value") {
      const kind = eq?.metadata?.kind;
      if (kind && SENSOR_KIND_LABELS[kind]) {
        rotulo = SENSOR_KIND_LABELS[kind];
      }
    }

    return `${eq?.tag || eqId} - ${rotulo}`;
  }, [equipments]);

  const getIcon = (type) => {
    switch(type) {
      case 'tank': return <Droplets size={18} />;
      case 'sensor': return <Thermometer size={18} />;
      case 'motor': return <Zap size={18} />;
      case 'valve': return <FlaskConical size={18} />;
      default: return <Box size={18} />;
    }
  };

  return (
    <>
      {/* Equipment Sidebar */}
      <aside
        className="eq-sidebar"
        style={{
          width: eqSidebarOpen ? 'var(--eq-sidebar-w)' : '0',
          opacity: eqSidebarOpen ? 1 : 0,
        }}
      >
        <div style={{ padding: '1.5rem', borderBottom: '1px solid var(--border)' }}>
          <h3 style={{ fontSize: '1.1rem' }}>Equipamentos</h3>
          <p className="label" style={{ fontSize: '0.65rem', marginTop: '4px' }}>Clique para ver, Ctrl+Clique para adicionar</p>
        </div>

        {Object.entries(groupedEquipments).map(([type, items]) => (
          <div key={type} className="category-group">
            <button className="category-header" onClick={() => toggleCategory(type)}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                {getIcon(type)}
                {CATEGORY_LABELS[type] || `${type}s`}
              </div>
              <ChevronRight size={16} style={{ transform: collapsedCats[type] ? 'rotate(0deg)' : 'rotate(90deg)', transition: '0.2s' }} />
            </button>

            {!collapsedCats[type] && (
              <div className="category-content">
                {(items as any[]).map(eq => (
                  <div
                    key={eq.id}
                    draggable={true}
                    onDragStart={(e) => onDragStart(e, eq.id)}
                    className={`eq-list-item ${selectedEqIds.includes(eq.id) ? 'active' : ''}`}
                    onClick={(e) => toggleEq(eq.id, e.ctrlKey)}
                  >
                    <div className="label" style={{ minWidth: '60px' }}>{eq.tag}</div>
                    <span style={{ fontSize: '0.8rem', opacity: 0.7 }}>{eq.id}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
        ))}
        <div style={{ height: '40px' }} />
      </aside>

      {/* Main Content */}
      <main
        className="main-content"
        style={{ minHeight: 'calc(100vh - 64px)' }}
        onDrop={onDrop}
        onDragOver={onDragOver}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', marginBottom: '2rem' }}>
          <button
            className="btn btn-ghost"
            style={{
              background: 'var(--card-bg)',
              border: '1px solid var(--border)',
              outline: 'none',
              boxShadow: 'none',
            }}
            onClick={() => setEqSidebarOpen(!eqSidebarOpen)}
          >
            {eqSidebarOpen ? <ChevronLeft size={20} /> : <ChevronRight size={20} />}
            <span style={{ fontSize: '0.9rem' }}>{eqSidebarOpen ? "Recolher Lista" : "Equipamentos"}</span>
          </button>

          <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
            {selectedEqIds.map(id => (
              <div key={id} className="btn" style={{ fontSize: '0.75rem', padding: '4px 10px', background: 'rgba(99, 102, 241, 0.1)', color: 'var(--primary)', border: '1px solid var(--primary)' }}>
                {id} <X size={12} style={{ marginLeft: '6px', cursor: 'pointer' }} onClick={() => toggleEq(id, true)} />
              </div>
            ))}
            {selectedEqIds.length > 1 && (
              <button className="btn-ghost" style={{ fontSize: '0.75rem' }} onClick={() => setSelectedEqIds([])}>Limpar</button>
            )}
          </div>
        </div>

        <div
          className="card"
          style={{ height: 'calc(100vh - 180px)' }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
            <h2 style={{ fontSize: '1.25rem' }}>Análise de Processo</h2>
            <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
              {zoomDomain && (
                <>
                  <div className="label" style={{ fontSize: '0.7rem', color: 'var(--primary)' }}>
                    {new Date(zoomDomain[0]).toLocaleTimeString()} — {new Date(zoomDomain[1]).toLocaleTimeString()}
                  </div>
                  <button
                    className="btn"
                    style={{
                      fontSize: '0.72rem',
                      padding: '5px 12px',
                      background: 'rgba(99, 102, 241, 0.12)',
                      color: 'var(--primary)',
                      border: '1px solid var(--primary)',
                    }}
                    onClick={resetZoom}
                  >
                    Resetar Período
                  </button>
                </>
              )}
              {activeYAxisKey && (
                <button
                  className="btn-ghost"
                  style={{ fontSize: '0.7rem', padding: '4px 8px', color: '#94a3b8' }}
                  onClick={() => setActiveYAxisKey(null)}
                >
                  Resetar Escala
                </button>
              )}
              <div className="label" style={{ opacity: 0.8 }}>Dados Sincronizados</div>
            </div>
          </div>

          <div style={{ width: '100%', height: 'calc(100% - 100px)' }}>
            <ResponsiveContainer width="100%" height="100%">
              <LineChart
                data={visibleData}
                onMouseDown={handleMouseDown}
                onMouseMove={handleMouseMove}
                onMouseUp={handleMouseUp}
                onMouseLeave={handleMouseUp}
                style={{ userSelect: 'none', cursor: isSelecting ? 'col-resize' : 'crosshair' }}
              >
                <CartesianGrid strokeDasharray="3 3" stroke="rgba(100,116,139,0.1)" vertical={false} />
                <XAxis
                  dataKey="ts"
                  type="number"
                  scale="time"
                  domain={zoomDomain ?? ['dataMin', 'dataMax']}
                  allowDataOverflow={true}
                  tickFormatter={(v: number) => new Date(v).toLocaleTimeString()}
                  minTickGap={60}
                  stroke="#94a3b8"
                  fontSize={11}
                  tickLine={false}
                  axisLine={false}
                  dy={10}
                />
                <YAxis
                  stroke="#94a3b8"
                  fontSize={11}
                  tickLine={false}
                  axisLine={false}
                  domain={yAxisDomain}
                  allowDataOverflow={true}
                />
                <Tooltip
                  contentStyle={{
                    background: 'var(--card-bg)',
                    border: '1px solid var(--border)',
                    borderRadius: '12px',
                    boxShadow: '0 10px 15px -3px rgba(0,0,0,0.1)',
                  }}
                  labelFormatter={(v: number) => new Date(v).toLocaleTimeString()}
                  formatter={(value: number, name: string) => {
                    return [typeof value === 'number' ? value.toFixed(2) : value, name];
                  }}
                />
                {seriesKeys.map((key, index) => {
                  const isAlarm = key.includes("::alarm_");
                  return (
                    <Line
                      key={key}
                      type={isAlarm ? "stepAfter" : "monotone"}
                      dataKey={key}
                      name={getSeriesLabel(key)}
                      stroke={COLORS[index % COLORS.length]}
                      strokeWidth={isAlarm ? 3 : (activeYAxisKey === key ? 3 : 2)}
                      strokeOpacity={activeYAxisKey && activeYAxisKey !== key ? 0.3 : 1}
                      dot={false}
                      activeDot={{ r: 5, strokeWidth: 0 }}
                      animationDuration={300}
                      connectNulls={false}
                    />
                  );
                })}
                {refAreaLeft !== null && refAreaRight !== null && (
                  <ReferenceArea
                    x1={refAreaLeft}
                    x2={refAreaRight}
                    strokeOpacity={0.3}
                    fill="var(--primary)"
                    fillOpacity={0.15}
                  />
                )}
              </LineChart>
            </ResponsiveContainer>
          </div>

          {/* Custom Legend with click and hover */}
          <div style={{
            display: 'flex',
            flexWrap: 'wrap',
            gap: '6px',
            justifyContent: 'center',
            paddingTop: '12px',
          }}>
            {seriesKeys.map((key, index) => {
              const isActive = activeYAxisKey === key;
              const isHovered = hoveredLegend === key;
              return (
                <div
                  key={key}
                  onClick={() => handleLegendClick(key)}
                  onMouseEnter={() => setHoveredLegend(key)}
                  onMouseLeave={() => setHoveredLegend(null)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '6px',
                    padding: '5px 12px',
                    borderRadius: '8px',
                    cursor: 'pointer',
                    fontSize: '0.75rem',
                    fontWeight: isActive ? 700 : 500,
                    color: isActive ? COLORS[index % COLORS.length] : (isHovered ? '#e2e8f0' : '#94a3b8'),
                    background: isActive
                      ? `${COLORS[index % COLORS.length]}20`
                      : (isHovered ? 'rgba(99, 102, 241, 0.08)' : 'transparent'),
                    border: isActive
                      ? `1px solid ${COLORS[index % COLORS.length]}40`
                      : '1px solid transparent',
                    transition: 'all 0.2s ease',
                    userSelect: 'none',
                  }}
                >
                  <div style={{
                    width: 10,
                    height: 10,
                    borderRadius: '50%',
                    background: COLORS[index % COLORS.length],
                    opacity: activeYAxisKey && !isActive ? 0.3 : 1,
                    transition: 'opacity 0.2s',
                  }} />
                  {getSeriesLabel(key)}
                </div>
              );
            })}
          </div>
        </div>
      </main>
    </>
  );
}
