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
} from "recharts";

const API_URL = "http://localhost:8080/api";

const COLORS = [
  "#6366f1", "#10b981", "#f59e0b", "#ef4444", "#8b5cf6", "#ec4899",
  "#14b8a6", "#f97316", "#06b6d4", "#84cc16", "#e879f9", "#fb923c"
];

const METRIC_LABELS: Record<string, string> = {
  volume_l: "Volume (L)",
  level_pct: "Nivel (%)",
  temp_c: "Temperatura (C)",
  pressure_bar: "Pressao (bar)",
  rpm: "RPM",
  is_open: "Aberta",
  active: "Ativo",
  value: "Valor",
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
    const timeMap: Record<string, any> = {};
    const keys = new Set<string>();

    (selectedEqIds as any[]).forEach(id => {
      const data = (allHistory as any)[id] || [];
      data.forEach((point: any) => {
        if (point.value === null || point.value === undefined) return;
        if (typeof point.value === 'string') return;

        const metric = point.metric || 'value';
        if (HIDDEN_METRICS.has(metric)) return;

        const timeStr = new Date(point.time).toLocaleTimeString();
        const seriesKey = `${id}::${metric}`;
        keys.add(seriesKey);

        if (!timeMap[timeStr]) timeMap[timeStr] = { time: timeStr };
        timeMap[timeStr][seriesKey] = point.value;
      });
    });

    return {
      chartData: Object.values(timeMap).sort((a: any, b: any) => a.time.localeCompare(b.time)),
      seriesKeys: Array.from(keys),
    };
  }, [allHistory, selectedEqIds]);

  // Compute Y-axis domain based on active legend selection
  const yAxisDomain = useMemo(() => {
    if (activeYAxisKey) {
      const [, metric] = activeYAxisKey.split("::");
      const range = METRIC_RANGES[metric];
      if (range) return range;
    }
    // Auto: compute from all visible data
    let min = Infinity;
    let max = -Infinity;
    seriesKeys.forEach(key => {
      chartData.forEach(row => {
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
  }, [activeYAxisKey, seriesKeys, chartData]);

  const handleLegendClick = useCallback((key: string) => {
    setActiveYAxisKey(prev => prev === key ? null : key);
  }, []);

  const getSeriesLabel = useCallback((key: string) => {
    const [eqId, metric] = key.split("::");
    const eq = (equipments as any[]).find(e => e.id === eqId);
    return `${eq?.tag || eqId} - ${METRIC_LABELS[metric] || metric}`;
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
                {type}s
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
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="rgba(100,116,139,0.1)" vertical={false} />
                <XAxis
                  dataKey="time"
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
