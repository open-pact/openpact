import type {ReactNode} from 'react';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FlowStep = {
  icon: ReactNode;
  label: string;
  description: string;
};

// Flow icons
function UserIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="32"
      height="32"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round">
      <circle cx="12" cy="8" r="5" />
      <path d="M20 21a8 8 0 1 0-16 0" />
    </svg>
  );
}

function DiscordIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="32"
      height="32"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round">
      <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
    </svg>
  );
}

function OpenPactIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="32"
      height="32"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round">
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
    </svg>
  );
}

function AIIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="32"
      height="32"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round">
      <path d="M12 8V4H8" />
      <rect width="16" height="12" x="4" y="8" rx="2" />
      <path d="M2 14h2" />
      <path d="M20 14h2" />
      <path d="M15 13v2" />
      <path d="M9 13v2" />
    </svg>
  );
}

function ToolsIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="32"
      height="32"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round">
      <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />
    </svg>
  );
}

function ResponseIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="32"
      height="32"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round">
      <path d="m3 21 1.9-5.7a8.5 8.5 0 1 1 3.8 3.8z" />
    </svg>
  );
}

function ArrowIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={styles.arrowIcon}>
      <path d="M5 12h14" />
      <path d="m12 5 7 7-7 7" />
    </svg>
  );
}

const flowSteps: FlowStep[] = [
  {
    icon: <UserIcon />,
    label: 'User',
    description: 'Sends message',
  },
  {
    icon: <DiscordIcon />,
    label: 'Discord',
    description: 'Chat interface',
  },
  {
    icon: <OpenPactIcon />,
    label: 'OpenPact',
    description: 'Routes securely',
  },
  {
    icon: <AIIcon />,
    label: 'AI Engine',
    description: 'Processes request',
  },
  {
    icon: <ToolsIcon />,
    label: 'MCP Tools',
    description: 'Approved actions',
  },
  {
    icon: <ResponseIcon />,
    label: 'Response',
    description: 'Secrets hidden',
  },
];

function FlowStep({icon, label, description}: FlowStep) {
  return (
    <div className={styles.flowStep}>
      <div className={styles.flowStepIcon}>{icon}</div>
      <div className={styles.flowStepLabel}>{label}</div>
      <div className={styles.flowStepDescription}>{description}</div>
    </div>
  );
}

export default function HowItWorks(): ReactNode {
  return (
    <section className={styles.howItWorks}>
      <div className="container">
        <div className={styles.sectionHeader}>
          <Heading as="h2" className={styles.sectionTitle}>
            How It Works
          </Heading>
          <p className={styles.sectionSubtitle}>
            OpenPact sits between you and your AI, ensuring security and control
            at every step.
          </p>
        </div>
        <div className={styles.flowContainer}>
          {flowSteps.map((step, idx) => (
            <div key={idx} className={styles.flowStepWrapper}>
              <FlowStep {...step} />
              {idx < flowSteps.length - 1 && <ArrowIcon />}
            </div>
          ))}
        </div>
        <div className={styles.highlights}>
          <div className={styles.highlight}>
            <span className={styles.highlightIcon}>&#10003;</span>
            <span>Secrets injected at runtime, never seen by AI</span>
          </div>
          <div className={styles.highlight}>
            <span className={styles.highlightIcon}>&#10003;</span>
            <span>All tool calls require explicit permission</span>
          </div>
          <div className={styles.highlight}>
            <span className={styles.highlightIcon}>&#10003;</span>
            <span>Full audit trail of all actions</span>
          </div>
        </div>
      </div>
    </section>
  );
}
