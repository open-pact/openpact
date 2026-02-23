import type {ReactNode} from 'react';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type UseCaseItem = {
  icon: ReactNode;
  title: string;
  description: string;
  features: string[];
};

// Use case icons
function PrivacyIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="36"
      height="36"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round">
      <rect width="18" height="11" x="3" y="11" rx="2" ry="2" />
      <path d="M7 11V7a5 5 0 0 1 10 0v4" />
    </svg>
  );
}

function DeveloperIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="36"
      height="36"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round">
      <polyline points="16 18 22 12 16 6" />
      <polyline points="8 6 2 12 8 18" />
    </svg>
  );
}

function TeamIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="36"
      height="36"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round">
      <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
      <circle cx="9" cy="7" r="4" />
      <path d="M22 21v-2a4 4 0 0 0-3-3.87" />
      <path d="M16 3.13a4 4 0 0 1 0 7.75" />
    </svg>
  );
}

function SelfHostIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="36"
      height="36"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round">
      <rect width="20" height="8" x="2" y="2" rx="2" ry="2" />
      <rect width="20" height="8" x="2" y="14" rx="2" ry="2" />
      <line x1="6" x2="6.01" y1="6" y2="6" />
      <line x1="6" x2="6.01" y1="18" y2="18" />
    </svg>
  );
}

const useCases: UseCaseItem[] = [
  {
    icon: <PrivacyIcon />,
    title: 'Privacy-Conscious Users',
    description: 'Keep your data local',
    features: [
      'All data stays on your machine',
      'No telemetry or tracking',
      'Full control over what AI sees',
      'Encrypted communications',
    ],
  },
  {
    icon: <DeveloperIcon />,
    title: 'Developers',
    description: 'Build custom integrations with Starlark',
    features: [
      'Python-like scripting syntax',
      'HTTP requests and JSON handling',
      'Safely sandboxed execution',
      'Easy to test and deploy',
    ],
  },
  {
    icon: <TeamIcon />,
    title: 'Teams',
    description: 'Audit and control what your AI can access',
    features: [
      'Approve scripts before execution',
      'Role-based access control',
      'Complete audit trail',
      'Centralized secret management',
    ],
  },
  {
    icon: <SelfHostIcon />,
    title: 'Self-Hosters',
    description: 'Full control over your AI stack',
    features: [
      'Docker-native deployment',
      'Prometheus metrics built-in',
      'Health checks and monitoring',
      'Use your own LLM provider',
    ],
  },
];

function UseCaseCard({icon, title, description, features}: UseCaseItem) {
  return (
    <div className={styles.useCaseCard}>
      <div className={styles.useCaseIcon}>{icon}</div>
      <Heading as="h3" className={styles.useCaseTitle}>
        {title}
      </Heading>
      <p className={styles.useCaseDescription}>{description}</p>
      <ul className={styles.useCaseFeatures}>
        {features.map((feature, idx) => (
          <li key={idx}>{feature}</li>
        ))}
      </ul>
    </div>
  );
}

export default function UseCases(): ReactNode {
  return (
    <section className={styles.useCases}>
      <div className="container">
        <div className={styles.sectionHeader}>
          <Heading as="h2" className={styles.sectionTitle}>
            Built for Everyone
          </Heading>
          <p className={styles.sectionSubtitle}>
            Whether you're an individual, developer, or enterprise team,
            OpenPact adapts to your needs.
          </p>
        </div>
        <div className={styles.useCaseGrid}>
          {useCases.map((useCase, idx) => (
            <UseCaseCard key={idx} {...useCase} />
          ))}
        </div>
      </div>
    </section>
  );
}
