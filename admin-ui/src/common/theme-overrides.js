// Light theme overrides
const themeOverrides = {
  common: {
    primaryColor: '#2db8b3',
    primaryColorHover: '#2dd7b7',
    primaryColorPressed: '#2387a7',
    primaryColorSuppl: '#2c96a7',
    errorColor: '#FF0055',
    warningColor: '#FF8000',
    borderRadius: '8px',
    borderRadiusSmall: '6px',
    borderColor: '#e4e7ec',
  },
  Card: {
    borderRadius: '12px',
  },
  Button: {
    borderRadiusMedium: '8px',
    borderRadiusSmall: '6px',
  },
  Menu: {
    borderRadius: '8px',
    itemBorderRadius: '8px',
  },
  Input: {
    borderRadius: '8px',
  },
  DataTable: {
    borderRadius: '0',
    tdColorHover: 'rgba(45, 184, 179, 0.04)',
  },
  Tag: {
    borderRadius: '6px',
  },
  List: {
    borderRadius: '0',
    borderColorPopover: '#e4e7ec',
  },
}

// Dark theme additions (merged with light overrides)
export const darkThemeOverrides = {
  common: {
    primaryColor: '#2db8b3',
    primaryColorHover: '#2dd7b7',
    primaryColorPressed: '#2387a7',
    primaryColorSuppl: '#2c96a7',
    errorColor: '#FF0055',
    warningColor: '#FF8000',
    borderRadius: '8px',
    borderRadiusSmall: '6px',
    borderColor: '#4b556eff',
    cardColor: '#202c4633',
    popoverColor: '#0f172a',
    modalColor: '#1c202c',
  },
  Card: {
    borderRadius: '12px',
    color: '#1c202c',
  },
  Button: {
    borderRadiusMedium: '8px',
    borderRadiusSmall: '6px',
  },
  Menu: {
    borderRadius: '8px',
    itemBorderRadius: '8px',
  },
  Input: {
    borderRadius: '8px',
  },
  Dropdown: {
    color: '#1c2334',
  },
  Drawer: {
    color: '#1c202c',
  },
  DataTable: {
    thColor: '#1c202c',
    tdColor: 'transparent',
    borderColor: '#20253578',
    borderRadius: '0',
    thColorHover: '#1c202c',
    tdColorHover: 'rgba(45, 184, 179, 0.04)',
  },
  Tag: {
    borderRadius: '6px',
  },
  List: {
    borderRadius: '0',
    borderColorPopover: '#1c2334',
    colorHoverPopover: '#1c202c',
  },
}

export default themeOverrides
