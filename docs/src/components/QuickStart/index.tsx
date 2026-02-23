import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

const dockerCommand = `docker run -d \\
  -p 8080:8080 \\
  -v openpact-workspace:/workspace \\
  -e DISCORD_TOKEN=your_token \\
  ghcr.io/open-pact/openpact:latest`;

function CopyIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round">
      <rect width="14" height="14" x="8" y="8" rx="2" ry="2" />
      <path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" />
    </svg>
  );
}

function TerminalIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round">
      <polyline points="4 17 10 11 4 5" />
      <line x1="12" x2="20" y1="19" y2="19" />
    </svg>
  );
}

export default function QuickStart(): ReactNode {
  const handleCopy = () => {
    navigator.clipboard.writeText(dockerCommand.replace(/\\\n/g, ' '));
  };

  return (
    <section className={styles.quickStart}>
      <div className="container">
        <div className={styles.sectionHeader}>
          <Heading as="h2" className={styles.sectionTitle}>
            Get Started in Seconds
          </Heading>
          <p className={styles.sectionSubtitle}>
            One command is all you need to run your own secure AI assistant.
          </p>
        </div>

        <div className={styles.codeContainer}>
          <div className={styles.codeHeader}>
            <div className={styles.terminalDots}>
              <span className={styles.dot} style={{backgroundColor: '#ff5f56'}} />
              <span className={styles.dot} style={{backgroundColor: '#ffbd2e'}} />
              <span className={styles.dot} style={{backgroundColor: '#27ca40'}} />
            </div>
            <div className={styles.terminalTitle}>
              <TerminalIcon />
              <span>Terminal</span>
            </div>
            <button
              className={styles.copyButton}
              onClick={handleCopy}
              title="Copy to clipboard"
              aria-label="Copy to clipboard">
              <CopyIcon />
            </button>
          </div>
          <pre className={styles.codeBlock}>
            <code>{dockerCommand}</code>
          </pre>
        </div>

        <div className={styles.steps}>
          <div className={styles.step}>
            <span className={styles.stepNumber}>1</span>
            <span className={styles.stepText}>Run the Docker command above</span>
          </div>
          <div className={styles.step}>
            <span className={styles.stepNumber}>2</span>
            <span className={styles.stepText}>Open the admin UI at localhost:8080 and complete setup</span>
          </div>
          <div className={styles.step}>
            <span className={styles.stepNumber}>3</span>
            <span className={styles.stepText}>Sign in to your LLM provider through the browser</span>
          </div>
          <div className={styles.step}>
            <span className={styles.stepNumber}>4</span>
            <span className={styles.stepText}>Start chatting with your AI!</span>
          </div>
        </div>

        <div className={styles.ctaContainer}>
          <Link
            className={styles.ctaButton}
            to="/docs/intro">
            Read the Full Guide
          </Link>
        </div>
      </div>
    </section>
  );
}
