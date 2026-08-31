import React, { createContext, useContext, useEffect, useState } from 'react'

export type Lang = 'fr' | 'en'

type Dict = Record<string, string>

const translations: Record<Lang, Dict> = {
  fr: {
    appTitle: 'Habit Tracker',
    admin: 'Admin',
    newHabit: '+ Nouvelle habitude',
    logout: 'Déconnexion',
    logoutTitle: 'Se déconnecter',
    loading: 'Chargement...',
    pageNotFound: 'Page non trouvée —',
    backHome: 'Retour accueil',
    seeArchived: 'Voir archivées',
    tip: 'Astuce: Clic +1 / toggle, appui long ou clic droit -1',
    globalProgress: 'Progression globale (cumulée)',
    globalDaily: 'Quotidienne',
    globalWeekly: 'Hebdomadaire',
    globalMonthly: 'Mensuelle',
    globalNoHabitDaily: 'Aucune habitude quotidienne',
    globalNoHabitWeekly: 'Aucune habitude hebdomadaire',
    globalNoHabitMonthly: 'Aucune habitude mensuelle',
    daily: 'Quotidien',
    weekly: 'Hebdomadaire',
    monthly: 'Mensuel',
    succeeded: 'réussies',
    avg: 'moyen',
    cumul: 'Cumul',
    allSucceeded: '✓ Toutes réussies',
    inProgress: 'En cours',
    toImprove: 'À améliorer',
    habit: 'Habitude',
    today: "Aujourd'hui",
    yesterday: 'Hier',
    dayMinus2: 'J-2',
    noGroup: 'Sans groupe',
    noHabit: 'Aucune habitude. Créez-en une !',
    cellHint: 'Clic +1 / toggle, appui long -1',
    administration: 'Administration',
    language: 'Langue',
    languageDesc: "Choisissez la langue de l'interface",
    french: 'Français',
    english: 'English',
    export: 'Export',
    exportDesc: 'Téléchargez les données au format CSV (compatible ré-import).',
    exportHabits: 'Export habits.csv',
    exportEntries: 'Export entries.csv',
    import: 'Import',
    importDescPrefix: 'Importez des fichiers CSV. Les habitudes sont upsert par',
    importDescMid: 'sinon',
    importDescEntries: 'Les entrées sont upsert par',
    formatDetails: 'Formats détaillés : voir',
    atRoot: 'à la racine du projet.',
    importHabitsBtn: 'Import habits',
    importEntriesBtn: 'Import entries',
    previewHabits: 'Aperçu format habits.csv',
    previewEntries: 'Aperçu format entries.csv',
    documentation: 'Documentation',
    docDesc: 'Le fichier',
    docDesc2: 'décrit exhaustivement les colonnes, valeurs par défaut, règles d\'upsert et exemples pour les deux imports.',
    objective: 'objectif',
    unitLabel: 'unité:',
    archived: 'Archivée',
    edit: 'Éditer',
    archive: 'Archiver',
    restore: 'Restaurer',
    delete: 'Supprimer',
    confirmDelete: 'Supprimer définitivement ?',
    progress: 'Progression',
    editDate: 'Éditer',
    done: 'Fait (✓)',
    save: 'Enregistrer',
    clear: 'Effacer',
    close: 'Fermer',
    group: 'Groupe',
    type: 'Type',
    editHabit: "Modifier l'habitude",
    createHabit: 'Nouvelle habitude',
    name: 'Nom',
    required: '*',
    groupLabel: 'Groupe',
    noGroupPlaceholder: 'Sans groupe si vide',
    typeLabel: 'Type',
    boolean: 'Booléen (oui/non)',
    numerical: 'Numérique',
    goal: 'Objectif',
    period: 'Période',
    dailyLong: 'Quotidien (jour J)',
    weeklyLong: 'Hebdomadaire (7 derniers jours)',
    monthlyLong: 'Mensuel (30 derniers jours)',
    unitOptional: 'Unité (optionnel)',
    unitPlaceholder: 'ex: km, pages, min, verres',
    negativeHabit: 'Habitude négative (seuil max à ne pas dépasser - succès si ≤ objectif)',
    create: 'Créer',
    cancel: 'Annuler',
    nameRequired: 'Nom requis',
    goalGt0: 'Objectif >0',
    todayBtn: "Aujourd'hui",
    mon: 'Lun',
    tue: 'Mar',
    wed: 'Mer',
    thu: 'Jeu',
    fri: 'Ven',
    sat: 'Sam',
    sun: 'Dim',
    login: 'Connexion',
    protectedInstance: 'Cette instance est protégée par mot de passe.',
    password: 'Mot de passe',
    passwordPlaceholder: 'Entrez le mot de passe',
    rememberPassword: 'Se souvenir du mot de passe',
    connecting: 'Connexion...',
    loginBtn: 'Se connecter',
    offline: 'Hors ligne — les données nécessitent une connexion',
    fileTooLarge: 'Fichier trop volumineux (max 10 MB)',
    importSuccess: 'importés avec succès.',
    imported: 'importés',
    errors: 'Erreurs',
  },
  en: {
    appTitle: 'Habit Tracker',
    admin: 'Admin',
    newHabit: '+ New habit',
    logout: 'Logout',
    logoutTitle: 'Log out',
    loading: 'Loading...',
    pageNotFound: 'Page not found —',
    backHome: 'Back to home',
    seeArchived: 'Show archived',
    tip: 'Tip: Click +1 / toggle, long press or right click -1',
    globalProgress: 'Overall progress (cumulative)',
    globalDaily: 'Daily',
    globalWeekly: 'Weekly',
    globalMonthly: 'Monthly',
    globalNoHabitDaily: 'No daily habits',
    globalNoHabitWeekly: 'No weekly habits',
    globalNoHabitMonthly: 'No monthly habits',
    daily: 'Daily',
    weekly: 'Weekly',
    monthly: 'Monthly',
    succeeded: 'succeeded',
    avg: 'avg',
    cumul: 'Total',
    allSucceeded: '✓ All succeeded',
    inProgress: 'In progress',
    toImprove: 'Needs improvement',
    habit: 'Habit',
    today: 'Today',
    yesterday: 'Yesterday',
    dayMinus2: 'D-2',
    noGroup: 'No group',
    noHabit: 'No habits. Create one!',
    cellHint: 'Click +1 / toggle, long press -1',
    administration: 'Administration',
    language: 'Language',
    languageDesc: 'Choose interface language',
    french: 'French',
    english: 'English',
    export: 'Export',
    exportDesc: 'Download data as CSV (compatible with re-import).',
    exportHabits: 'Export habits.csv',
    exportEntries: 'Export entries.csv',
    import: 'Import',
    importDescPrefix: 'Import CSV files. Habits are upserted by',
    importDescMid: 'otherwise',
    importDescEntries: 'Entries are upserted by',
    formatDetails: 'Detailed formats: see',
    atRoot: 'at project root.',
    importHabitsBtn: 'Import habits',
    importEntriesBtn: 'Import entries',
    previewHabits: 'Preview habits.csv format',
    previewEntries: 'Preview entries.csv format',
    documentation: 'Documentation',
    docDesc: 'The file',
    docDesc2: 'describes exhaustively columns, defaults, upsert rules and examples for both imports.',
    objective: 'goal',
    unitLabel: 'unit:',
    archived: 'Archived',
    edit: 'Edit',
    archive: 'Archive',
    restore: 'Restore',
    delete: 'Delete',
    confirmDelete: 'Delete permanently?',
    progress: 'Progress',
    editDate: 'Edit',
    done: 'Done (✓)',
    save: 'Save',
    clear: 'Clear',
    close: 'Close',
    group: 'Group',
    type: 'Type',
    editHabit: 'Edit habit',
    createHabit: 'New habit',
    name: 'Name',
    required: '*',
    groupLabel: 'Group',
    noGroupPlaceholder: 'No group if empty',
    typeLabel: 'Type',
    boolean: 'Boolean (yes/no)',
    numerical: 'Numerical',
    goal: 'Goal',
    period: 'Period',
    dailyLong: 'Daily (today)',
    weeklyLong: 'Weekly (last 7 days)',
    monthlyLong: 'Monthly (last 30 days)',
    unitOptional: 'Unit (optional)',
    unitPlaceholder: 'e.g. km, pages, min, glasses',
    negativeHabit: 'Negative habit (max threshold - success if ≤ goal)',
    create: 'Create',
    cancel: 'Cancel',
    nameRequired: 'Name required',
    goalGt0: 'Goal >0',
    todayBtn: 'Today',
    mon: 'Mon',
    tue: 'Tue',
    wed: 'Wed',
    thu: 'Thu',
    fri: 'Fri',
    sat: 'Sat',
    sun: 'Sun',
    login: 'Login',
    protectedInstance: 'This instance is password protected.',
    password: 'Password',
    passwordPlaceholder: 'Enter password',
    rememberPassword: 'Remember password',
    connecting: 'Connecting...',
    loginBtn: 'Log in',
    offline: 'Offline — data requires connection',
    fileTooLarge: 'File too large (max 10 MB)',
    importSuccess: 'imported successfully.',
    imported: 'imported',
    errors: 'Errors',
  },
}

const LANG_KEY = 'app_lang'

function getInitialLang(): Lang {
  try {
    const stored = localStorage.getItem(LANG_KEY)
    if (stored === 'en' || stored === 'fr') return stored
  } catch {}
  // default french as requested
  return 'fr'
}

type Ctx = {
  lang: Lang
  setLang: (l: Lang) => void
  t: (key: string) => string
}

const I18nContext = createContext<Ctx>({
  lang: 'fr',
  setLang: () => {},
  t: (k) => k,
})

export function LanguageProvider({ children }: { children: React.ReactNode }) {
  const [lang, setLangState] = useState<Lang>(getInitialLang)

  const setLang = (l: Lang) => {
    setLangState(l)
    try { localStorage.setItem(LANG_KEY, l) } catch {}
    try { document.documentElement.lang = l } catch {}
  }

  useEffect(() => {
    try {
      const s = localStorage.getItem(LANG_KEY)
      if (s === 'en' || s === 'fr') setLangState(s)
    } catch {}
  }, [])

  useEffect(() => {
    try { document.documentElement.lang = lang } catch {}
  }, [lang])

  const t = (key: string) => {
    const d = translations[lang]
    if (d[key] !== undefined) return d[key]
    if (translations.fr[key] !== undefined) return translations.fr[key]
    // fallback: return key and warn in dev
    if (typeof window !== 'undefined' && (import.meta as any).env?.DEV) {
      console.warn(`Missing translation for key: ${key} (lang: ${lang})`)
    }
    return key
  }

  return <I18nContext.Provider value={{ lang, setLang, t }}>{children}</I18nContext.Provider>
}

export function useI18n() {
  return useContext(I18nContext)
}
