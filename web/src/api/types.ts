import type { components as Auth } from "./generated/auth";
import type { components as Client } from "./generated/client";
import type { components as Bank } from "./generated/bank";

type C = Client["schemas"];
type B = Bank["schemas"];

export type AccessToken = Auth["schemas"]["AccessTokenResponse"];
export type CurrentUser = Auth["schemas"]["CurrentUser"];

export type Money = C["Money"];
export type SignedMoney = C["SignedMoney"];
export type Problem = C["Problem"];
export type PurposeTag = C["PurposeTag"];
export type DocumentType = C["DocumentType"];
export type ClientStatus = C["ClientStatus"];
export type BankStatus = B["BankCase"]["status"];
export type CompanyProfile = C["CompanyProfile"];
export type VersionedCompany = C["VersionedCompany"];
export type Product = C["Product"];
export type DocumentMetadata = C["DocumentMetadata"];
export type InfoRequest = C["InfoRequest"];
export type InfoField = C["InfoField"];
export type Offer = C["Offer"];
export type ClientApplication = C["ClientApplication"];
export type ApplicationForm = C["ApplicationForm"];
export type ApplicationFormPatch = C["ApplicationFormPatch"];
export type ApplicationListItem = C["ApplicationListItem"];
export type ApplicationPage = C["ApplicationPage"];
export type ProvideInfoRequest = C["ProvideInfoRequest"];
export type TimelineEntry = C["TimelineEntry"];
export type ClientAction = ClientApplication["allowed_actions"][number];

export type BankCase = B["BankCase"];
export type CaseListItem = B["CaseListItem"];
export type CasePage = B["CasePage"];
export type Checks = B["Checks"];
export type ScoreResult = B["ScoreResult"];
export type ScoreReason = B["ScoreReason"];
export type AnalystDecisionRequest = B["AnalystDecisionRequest"];
export type AuditEntry = B["AuditEntry"];
export type AuditPage = B["AuditPage"];
export type CaseSummary = { text: string; source: "assistant" | "template" };
