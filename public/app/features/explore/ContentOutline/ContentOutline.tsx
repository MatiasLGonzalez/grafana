import { css } from '@emotion/css';
import React from 'react';
import { useToggle } from 'react-use';

import { GrafanaTheme2 } from '@grafana/data';
import { reportInteraction } from '@grafana/runtime';
import { useStyles2 } from '@grafana/ui';

import { useContentOutlineContext } from './ContentOutlineContext';
import { ContentOutlineItemButton } from './ContentOutlineItemButton';

const getStyles = (theme: GrafanaTheme2) => {
  return {
    wrapper: css({
      label: 'wrapper',
      position: 'relative',
      display: 'flex',
      justifyContent: 'center',
      marginRight: theme.spacing(1),
      height: '100%',
      backgroundColor: theme.colors.background.primary,
    }),
    content: css({
      label: 'content',
      position: 'sticky',
      top: '56px',
      height: '81vh',
    }),
    buttonStyles: css({
      textAlign: 'left',
      width: '100%',
    }),
  };
};

export function ContentOutline() {
  const [expanded, toggleExpanded] = useToggle(false);
  const styles = useStyles2((theme) => getStyles(theme));
  const { outlineItems } = useContentOutlineContext();

  const scrollIntoView = (ref: HTMLElement | null, buttonTitle: string) => {
    ref?.scrollIntoView({ behavior: 'smooth' });
    reportInteraction('explore_toolbar_contentoutline_clicked', {
      item: 'select_section',
      type: buttonTitle,
    });
  };

  const toggle = () => {
    toggleExpanded();
    reportInteraction('explore_toolbar_contentoutline_clicked', {
      item: 'outline',
      type: expanded ? 'minimize' : 'expand',
    });
  };

  return (
    <div className={styles.wrapper} id="content-outline-container">
      <div className={styles.content}>
        <ContentOutlineItemButton
          title={expanded ? 'Collapse content outline' : undefined}
          icon={expanded ? 'angle-left' : 'angle-right'}
          onClick={toggle}
          tooltip={!expanded ? 'Expand content outline' : undefined}
          className={styles.buttonStyles}
          aria-expanded={expanded}
        />

        {outlineItems.map((item) => (
          <ContentOutlineItemButton
            key={item.id}
            title={expanded ? item.title : undefined}
            className={styles.buttonStyles}
            icon={item.icon}
            onClick={() => scrollIntoView(item.ref, item.title)}
            tooltip={!expanded ? item.title : undefined}
          />
        ))}
      </div>
    </div>
  );
}