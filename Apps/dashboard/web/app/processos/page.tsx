// @ts-nocheck
"use client";

export default function ProcessosPage() {
  return (
    <main className="main-content" style={{ gridColumn: '2 / -1', minHeight: 'calc(100vh - 64px)', padding: '2rem', overflow: 'auto' }}>
      <div className="card" style={{ padding: '2rem' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '2rem' }}>
          <h2 style={{ fontSize: '1.25rem' }}>Diagrama de Processo (P&ID)</h2>
          <div className="label" style={{ opacity: 0.8 }}>TCC - Sistema de Tanque Aquecido</div>
        </div>

        <div style={{ width: '100%', display: 'flex', justifyContent: 'center' }}>
          <PlantDiagram />
        </div>
      </div>
    </main>
  );
}

function PlantDiagram() {
  const pipe = "#6366f1";
  const pw = 5;
  const tank = "#4f46e5";
  const valve = "#10b981";
  const motor = "#f59e0b";
  const sensor = "#ef4444";
  const txt = "#e2e8f0";
  const dim = "#64748b";

  return (
    <svg viewBox="0 0 1000 850" width="100%" style={{ maxWidth: '1000px', background: '#0a0a14', borderRadius: '16px', border: '1px solid rgba(99,102,241,0.15)' }}>
      <defs>
        <marker id="arrow" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
          <polygon points="0 0, 8 3, 0 6" fill={dim} />
        </marker>
      </defs>

      {/* ========== TOP INLET PIPES ========== */}

      {/* Produto A (right side, top) → XV-01 → tank */}
      <text x="920" y="115" fill={dim} fontSize="11" textAnchor="end">Produto A</text>
      <text x="920" y="128" fill="#475569" fontSize="9" textAnchor="end">30 L/min, 25°C</text>
      <line x1="870" y1="120" x2="770" y2="120" stroke={pipe} strokeWidth={pw} markerStart="url(#arrow)" />
      <ValveH x={720} y={110} label="XV-01" color={valve} txt={txt} />
      <line x1="700" y1="120" x2="600" y2="120" stroke={pipe} strokeWidth={pw} />
      <line x1="600" y1="120" x2="600" y2="250" stroke={pipe} strokeWidth={pw} />

      {/* CIP → XV-02 → tank */}
      <text x="920" y="185" fill={dim} fontSize="11" textAnchor="end">CIP</text>
      <text x="920" y="198" fill="#475569" fontSize="9" textAnchor="end">25 L/min, 70°C</text>
      <line x1="870" y1="190" x2="770" y2="190" stroke={pipe} strokeWidth={pw} markerStart="url(#arrow)" />
      <ValveH x={720} y={180} label="XV-02" color={valve} txt={txt} />
      <line x1="700" y1="190" x2="570" y2="190" stroke={pipe} strokeWidth={pw} />
      <line x1="570" y1="190" x2="570" y2="250" stroke={pipe} strokeWidth={pw} />

      {/* Produto B → XV-03 → tank */}
      <text x="920" y="255" fill={dim} fontSize="11" textAnchor="end">Produto B</text>
      <text x="920" y="268" fill="#475569" fontSize="9" textAnchor="end">30 L/min, 15°C</text>
      <line x1="870" y1="260" x2="770" y2="260" stroke={pipe} strokeWidth={pw} markerStart="url(#arrow)" />
      <ValveH x={720} y={250} label="XV-03" color={valve} txt={txt} />
      <line x1="700" y1="260" x2="630" y2="260" stroke={pipe} strokeWidth={pw} />
      <line x1="630" y1="260" x2="630" y2="300" stroke={pipe} strokeWidth={pw} />

      {/* Ar Limpo → XV-04 → tank headspace */}
      <text x="920" y="375" fill={dim} fontSize="11" textAnchor="end">Ar Limpo</text>
      <text x="920" y="388" fill="#475569" fontSize="9" textAnchor="end">120 Nlpm, 1.2 bar</text>
      <line x1="870" y1="380" x2="770" y2="380" stroke={pipe} strokeWidth={pw} markerStart="url(#arrow)" />
      <ValveH x={720} y={370} label="XV-04" color={valve} txt={txt} />
      <line x1="700" y1="380" x2="630" y2="380" stroke={pipe} strokeWidth={pw} />
      <line x1="630" y1="380" x2="630" y2="340" stroke={pipe} strokeWidth={pw} />

      {/* ========== TANK TK-01 ========== */}
      {/* Tank body */}
      <rect x="420" y="250" width="220" height="340" rx="12" fill="none" stroke={tank} strokeWidth="3" />
      <rect x="420" y="250" width="220" height="340" rx="12" fill={tank} fillOpacity="0.06" />

      {/* Tank top dome */}
      <path d="M 420 265 Q 530 220 640 265" fill="none" stroke={tank} strokeWidth="3" />

      {/* Product level */}
      <rect x="430" y="420" width="200" height="160" rx="6" fill={tank} fillOpacity="0.12" />
      <text x="530" y="500" fill={pipe} fontSize="11" textAnchor="middle" opacity="0.4">Produto</text>

      {/* Tank label */}
      <text x="530" y="290" fill={txt} fontSize="16" fontWeight="bold" textAnchor="middle">TK-01</text>
      <text x="530" y="308" fill={dim} fontSize="10" textAnchor="middle">1000 L</text>

      {/* ========== AGITATOR AG-01 ========== */}
      {/* Motor housing on top */}
      <rect x="505" y="215" width="50" height="35" rx="4" fill="#1a1840" stroke={motor} strokeWidth="2" />
      <text x="530" y="228" fill={motor} fontSize="10" fontWeight="bold" textAnchor="middle">M</text>
      <text x="530" y="243" fill={motor} fontSize="12" fontWeight="bold" textAnchor="middle">G</text>
      <text x="570" y="228" fill={txt} fontSize="10">AG-01</text>

      {/* Shaft */}
      <line x1="530" y1="250" x2="530" y2="380" stroke={motor} strokeWidth="2" />

      {/* Propeller blades */}
      <line x1="530" y1="380" x2="500" y2="405" stroke={motor} strokeWidth="3" strokeLinecap="round" />
      <line x1="530" y1="380" x2="560" y2="405" stroke={motor} strokeWidth="3" strokeLinecap="round" />
      <line x1="530" y1="395" x2="505" y2="370" stroke={motor} strokeWidth="3" strokeLinecap="round" />
      <line x1="530" y1="395" x2="555" y2="370" stroke={motor} strokeWidth="3" strokeLinecap="round" />

      {/* ========== LEFT SIDE: VAPOR + VENT ========== */}

      {/* XV-05 Vent → Atmosfera */}
      <text x="140" y="310" fill={dim} fontSize="11">atmosfera</text>
      <line x1="210" y1="320" x2="280" y2="320" stroke={pipe} strokeWidth={pw} markerEnd="url(#arrow)" />
      <ValveH x={310} y={310} label="XV-05" color={valve} txt={txt} />
      <line x1="340" y1="320" x2="420" y2="320" stroke={pipe} strokeWidth={pw} />

      {/* XV-06 Steam → Vapor */}
      <text x="80" y="440" fill={dim} fontSize="11">Vapor</text>
      <text x="80" y="453" fill="#475569" fontSize="9">140°C</text>
      <line x1="120" y1="445" x2="200" y2="445" stroke={pipe} strokeWidth={pw} markerStart="url(#arrow)" />
      <ValveH x={230} y={435} label="XV-06" color={valve} txt={txt} />
      <line x1="260" y1="445" x2="340" y2="445" stroke={pipe} strokeWidth={pw} />
      {/* Steam jacket indicator (dashed around tank) */}
      <rect x="410" y="370" width="240" height="180" rx="14" fill="none" stroke="#ef4444" strokeWidth="1.5" strokeDasharray="6 4" opacity="0.3" />
      <line x1="340" y1="445" x2="410" y2="445" stroke={pipe} strokeWidth={pw} />

      {/* TIT-02 sensor (steam temp) */}
      <circle cx="170" cy="490" r="22" fill="none" stroke={sensor} strokeWidth="2" />
      <text x="170" y="486" fill={txt} fontSize="9" fontWeight="bold" textAnchor="middle">TIT</text>
      <text x="170" y="499" fill={txt} fontSize="9" fontWeight="bold" textAnchor="middle">02</text>
      <line x1="170" y1="468" x2="170" y2="445" stroke={sensor} strokeWidth="1.5" strokeDasharray="4 2" />

      {/* ========== RIGHT SIDE: SENSORS ========== */}

      {/* Sensor group - inside/beside tank */}
      {/* TIT-01 */}
      <SensorBox x={660} y={330} tag1="TIT" tag2="01" color={sensor} txt={txt} />
      <line x1="640" y1="345" x2="660" y2="345" stroke={sensor} strokeWidth="1.5" strokeDasharray="4 2" />

      {/* TI-01 (indicator) */}
      <SensorBoxSquare x={720} y={330} tag1="TI" tag2="01" color="#06b6d4" txt={txt} />

      {/* PIT-01 */}
      <SensorBox x={660} y={400} tag1="PIT" tag2="01" color={sensor} txt={txt} />
      <line x1="640" y1="415" x2="660" y2="415" stroke={sensor} strokeWidth="1.5" strokeDasharray="4 2" />

      {/* PIT indicator */}
      <SensorBoxSquare x={720} y={400} tag1="PIT" tag2="01" color="#06b6d4" txt={txt} />

      {/* LIT-01 (level) */}
      <SensorBox x={660} y={470} tag1="LV" tag2="01" color={sensor} txt={txt} />
      <line x1="640" y1="485" x2="660" y2="485" stroke={sensor} strokeWidth="1.5" strokeDasharray="4 2" />

      {/* LV indicator */}
      <SensorBoxSquare x={720} y={470} tag1="LV" tag2="01" color="#06b6d4" txt={txt} />

      {/* ========== BOTTOM: OUTLET ========== */}

      {/* Tank bottom outlet pipe */}
      <line x1="530" y1="590" x2="530" y2="650" stroke={pipe} strokeWidth={pw} />

      {/* XV-07 Transfer valve */}
      <ValveV x={520} y={650} label="XV-07" color={valve} txt={txt} />

      {/* Pipe to pump */}
      <line x1="530" y1="690" x2="530" y2="730" stroke={pipe} strokeWidth={pw} />

      {/* MT-01 Transfer pump */}
      <text x="465" y="750" fill={txt} fontSize="10">MT-01</text>
      <circle cx="530" cy="760" r="25" fill="#1a1840" stroke={motor} strokeWidth="2" />
      {/* Pump impeller symbol */}
      <path d="M 520 750 Q 530 740 540 750 Q 530 760 520 750" fill="none" stroke={motor} strokeWidth="2" />

      {/* Outlet pipe */}
      <line x1="555" y1="760" x2="680" y2="760" stroke={pipe} strokeWidth={pw} />
      <polygon points="670,754 680,760 670,766" fill={pipe} />
      <text x="700" y="755" fill={dim} fontSize="11">Transferencia</text>

      {/* Left side return */}
      <text x="80" y="575" fill={dim} fontSize="11">Retorno</text>
      <line x1="140" y1="578" x2="200" y2="578" stroke={pipe} strokeWidth={pw} markerEnd="url(#arrow)" />
      <line x1="200" y1="578" x2="420" y2="578" stroke={pipe} strokeWidth={pw} />

      {/* ========== DASHED SIGNAL LINES ========== */}
      {/* Dashed lines from sensors to instruments */}
      <line x1="695" y1="345" x2="720" y2="345" stroke={dim} strokeWidth="1" strokeDasharray="3 2" />
      <line x1="695" y1="415" x2="720" y2="415" stroke={dim} strokeWidth="1" strokeDasharray="3 2" />
      <line x1="695" y1="485" x2="720" y2="485" stroke={dim} strokeWidth="1" strokeDasharray="3 2" />

      {/* ========== GRID LINES (background) ========== */}
      {/* Subtle grid for engineering feel */}
      {Array.from({ length: 20 }).map((_, i) => (
        <line key={`vg${i}`} x1={i * 50} y1="0" x2={i * 50} y2="850" stroke="#1e1b4b" strokeWidth="0.5" opacity="0.3" />
      ))}
      {Array.from({ length: 17 }).map((_, i) => (
        <line key={`hg${i}`} x1="0" y1={i * 50} x2="1000" y2={i * 50} stroke="#1e1b4b" strokeWidth="0.5" opacity="0.3" />
      ))}
    </svg>
  );
}

function ValveH({ x, y, label, color, txt }) {
  // Horizontal valve (bowtie shape, flow left-right)
  return (
    <g>
      <polygon points={`${x-20},${y} ${x},${y+10} ${x-20},${y+20}`} fill="none" stroke={color} strokeWidth="2" />
      <polygon points={`${x+20},${y} ${x},${y+10} ${x+20},${y+20}`} fill="none" stroke={color} strokeWidth="2" />
      <rect x={x-4} y={y-2} width="8" height="8" fill="#1a1840" stroke={color} strokeWidth="1.5" />
      <text x={x} y={y-8} fill={txt} fontSize="9" fontWeight="bold" textAnchor="middle">{label}</text>
    </g>
  );
}

function ValveV({ x, y, label, color, txt }) {
  // Vertical valve (bowtie shape, flow top-bottom)
  return (
    <g>
      <polygon points={`${x},${y-15} ${x+10},${y} ${x+20},${y-15}`} fill="none" stroke={color} strokeWidth="2" />
      <polygon points={`${x},${y+15} ${x+10},${y} ${x+20},${y+15}`} fill="none" stroke={color} strokeWidth="2" />
      <rect x={x+6} y={y-4} width="8" height="8" fill="#1a1840" stroke={color} strokeWidth="1.5" />
      <text x={x+35} y={y+4} fill={txt} fontSize="9" fontWeight="bold">{label}</text>
    </g>
  );
}

function SensorBox({ x, y, tag1, tag2, color, txt }) {
  // Round sensor transmitter (circle with tag)
  return (
    <g>
      <circle cx={x+17} cy={y+15} r="18" fill="#0f0d2e" stroke={color} strokeWidth="2" />
      <text x={x+17} y={y+12} fill={color} fontSize="9" fontWeight="bold" textAnchor="middle">{tag1}</text>
      <text x={x+17} y={y+23} fill={color} fontSize="9" fontWeight="bold" textAnchor="middle">{tag2}</text>
    </g>
  );
}

function SensorBoxSquare({ x, y, tag1, tag2, color, txt }) {
  // Square indicator/display
  return (
    <g>
      <rect x={x} y={y} width="36" height="30" rx="3" fill="#0f0d2e" stroke={color} strokeWidth="1.5" />
      {/* Diagonal line for indicator symbol */}
      <line x1={x} y1={y+30} x2={x+36} y2={y} stroke={color} strokeWidth="0.8" opacity="0.5" />
      <text x={x+18} y={y+13} fill={color} fontSize="8" fontWeight="bold" textAnchor="middle">{tag1}</text>
      <text x={x+18} y={y+24} fill={color} fontSize="8" fontWeight="bold" textAnchor="middle">{tag2}</text>
    </g>
  );
}
