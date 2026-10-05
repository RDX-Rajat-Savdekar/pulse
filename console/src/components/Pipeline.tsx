import { useEffect, useRef, useState } from "react";
import gsap from "gsap";
import { useGSAP } from "@gsap/react";
import {
  Handle,
  Position,
  ReactFlow,
  useReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

gsap.registerPlugin(useGSAP);

type StationData = { label: string; source?: "right" | "bottom"; target?: "left" | "top" };
type StationNode = Node<StationData, "station">;

const acceptedOrder = ["ingest", "redis", "kafka", "processor", "postgres", "graphql"];
const duplicateOrder = ["ingest", "redis"];

const stationHeight = 30;

function station(id: string, label: string, x: number, y: number, width: number): StationNode {
  return {
    id,
    position: { x, y },
    data: { label },
    type: "station",
    width,
    height: stationHeight,
    handles: [
      { type: "target", position: Position.Left, x: 0, y: stationHeight / 2, width: 1, height: 1 },
      { type: "source", position: Position.Right, x: width, y: stationHeight / 2, width: 1, height: 1 },
    ],
  };
}

const wideNodes: StationNode[] = [
  station("ingest", "Ingest", 8, 28, 72),
  station("redis", "Redis", 128, 28, 64),
  station("kafka", "Kafka", 240, 28, 64),
  station("processor", "Processor", 352, 28, 92),
  station("postgres", "Postgres", 492, 28, 84),
  station("graphql", "GraphQL", 624, 28, 76),
];

const narrowNodes: StationNode[] = [
  station("ingest", "Ingest", 4, 4, 72),
  station("redis", "Redis", 112, 4, 64),
  station("kafka", "Kafka", 212, 4, 64),
  station("processor", "Processor", 4, 74, 92),
  station("postgres", "Postgres", 132, 74, 84),
  station("graphql", "GraphQL", 252, 74, 76),
];

narrowNodes[2].data = { label: "Kafka", source: "bottom" };
narrowNodes[2].handles = [
  { type: "target", position: Position.Left, x: 0, y: stationHeight / 2, width: 1, height: 1 },
  { type: "source", position: Position.Bottom, x: 32, y: stationHeight, width: 1, height: 1 },
];
narrowNodes[3].data = { label: "Processor", target: "top" };
narrowNodes[3].handles = [
  { type: "target", position: Position.Top, x: 46, y: 0, width: 1, height: 1 },
  { type: "source", position: Position.Right, x: 92, y: stationHeight / 2, width: 1, height: 1 },
];

const stroke = { stroke: "#6d8290", strokeWidth: 1.25 };

const edges: Edge[] = [
  ["ingest", "redis"],
  ["redis", "kafka"],
  ["kafka", "processor"],
  ["processor", "postgres"],
  ["postgres", "graphql"],
].map(([source, target]) => ({
  id: `${source}-${target}`,
  source,
  target,
  type: "straight",
  style: stroke,
}));

function Station({ data }: NodeProps<StationNode>) {
  const source = data.source === "bottom" ? Position.Bottom : Position.Right;
  const target = data.target === "top" ? Position.Top : Position.Left;
  return (
    <div className="station" data-station={data.label.toLowerCase()}>
      <Handle type="target" position={target} />
      <span>{data.label}</span>
      <Handle type="source" position={source} />
    </div>
  );
}

const nodeTypes = { station: Station };

function Refit({ layout }: { layout: number }) {
  const { fitView } = useReactFlow();
  useEffect(() => {
    const run = () => fitView({ padding: 0.14, duration: 0 });
    const frame = requestAnimationFrame(run);
    const later = window.setTimeout(run, 60);
    return () => {
      cancelAnimationFrame(frame);
      window.clearTimeout(later);
    };
  }, [layout, fitView]);
  return null;
}

export default function Pipeline({ tick, kind }: { tick: number; kind: string }) {
  const root = useRef<HTMLDivElement>(null);
  const [layout, setLayout] = useState(wideNodes);

  useGSAP(() => {
    if (!tick || !root.current) return;
    const order = kind === "duplicate" ? duplicateOrder : acceptedOrder;
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const stations = order
      .map((id) => root.current?.querySelector(`[data-id="${id}"] .station`))
      .filter((node): node is HTMLElement => node instanceof HTMLElement);
    const bead = root.current.querySelector(".bead");
    if (reduce) {
      stations.forEach((station) => station.classList.add("hot"));
      window.setTimeout(() => stations.forEach((station) => station.classList.remove("hot")), 700);
      return;
    }
    const box = root.current.getBoundingClientRect();
    const points = stations.map((station) => {
      const rect = station.getBoundingClientRect();
      return {
        x: rect.left - box.left + rect.width / 2 - 3.5,
        y: rect.bottom - box.top - 4,
      };
    });
    const timeline = gsap.timeline();
    stations.forEach((station, index) => {
      timeline.call(() => station.classList.add("hot"), undefined, index * 0.16);
    });
    if (bead && points.length > 1) {
      gsap.set(bead, { x: points[0].x, y: points[0].y, opacity: 1 });
      points.slice(1).forEach((point, index) => {
        timeline.to(bead, { x: point.x, y: point.y, duration: 0.16, ease: "power1.inOut" }, index * 0.16);
      });
      timeline.to(bead, { opacity: 0, duration: 0.15 });
    }
    timeline.call(() => stations.forEach((station) => station.classList.remove("hot")), undefined, "+=0.35");
  }, { scope: root, dependencies: [tick, kind], revertOnUpdate: true });

  useEffect(() => {
    const query = window.matchMedia("(max-width: 639px)");
    const apply = () => setLayout(query.matches ? narrowNodes : wideNodes);
    apply();
    query.addEventListener("change", apply);
    window.addEventListener("resize", apply);
    return () => {
      query.removeEventListener("change", apply);
      window.removeEventListener("resize", apply);
    };
  }, []);

  return (
    <div className="flow" ref={root}>
      <span className="bead" />
      <ReactFlow
        key={layout[3].position.y}
        nodes={layout}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.16 }}
        minZoom={0.55}
        maxZoom={1.25}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        panOnDrag={false}
        zoomOnScroll={false}
        zoomOnPinch={false}
        zoomOnDoubleClick={false}
        proOptions={{ hideAttribution: true }}
      >
        <Refit layout={layout[3].position.y} />
      </ReactFlow>
    </div>
  );
}
