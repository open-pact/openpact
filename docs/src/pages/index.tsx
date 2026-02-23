import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import HomepageFeatures from '@site/src/components/HomepageFeatures';
import HowItWorks from '@site/src/components/HowItWorks';
import UseCases from '@site/src/components/UseCases';
import QuickStart from '@site/src/components/QuickStart';
import Heading from '@theme/Heading';

import styles from './index.module.css';

function ComingSoonBanner() {
  return (
    <div className={styles.comingSoonBanner}>
      <div className="container">
        <span className={styles.comingSoonBadge}>Coming Soon</span>
        <p className={styles.comingSoonText}>
          OpenPact is currently in <strong>Beta</strong> and is not yet publicly available.
          Interested in beta testing? Email us at{' '}
          <a href="mailto:hello@openpact.ai" className={styles.comingSoonLink}>
            hello@openpact.ai
          </a>
        </p>
      </div>
    </div>
  );
}

function HomepageHeader() {
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <Heading as="h1" className={styles.heroTitle}>
          Your AI Assistant, Your Rules
        </Heading>
        <p className={styles.heroSubtitle}>
          OpenPact is a secure, open-source framework for running AI assistants
          with sandboxed capabilities and complete data privacy.
        </p>
        <div className={styles.buttons}>
          <Link
            className={clsx('button button--lg', styles.primaryButton)}
            to="/docs/intro">
            Get Started
          </Link>
          <Link
            className={clsx('button button--lg', styles.secondaryButton)}
            href="https://github.com/open-pact/openpact">
            View on GitHub
          </Link>
        </div>
      </div>
    </header>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title="Your AI Assistant, Your Rules"
      description="OpenPact is a secure, open-source framework for running AI assistants with sandboxed capabilities and complete data privacy.">
      <HomepageHeader />
      <ComingSoonBanner />
      <main>
        <HomepageFeatures />
        <HowItWorks />
        <UseCases />
        <QuickStart />
      </main>
    </Layout>
  );
}
