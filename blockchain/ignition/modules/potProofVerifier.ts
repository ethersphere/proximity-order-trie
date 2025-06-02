import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

/**
 * Ignition module for deploying the POTProofVerifier library
 * 
 * This module handles the deployment of:
 * - BMTChunk library
 * - POTProofVerifier library
 */
const potProofVerifierModule = buildModule("POTProofVerifier", (m) => {
  const potProofVerifier = m.library("POTProofVerifier");

  return { potProofVerifier };
});

export default potProofVerifierModule;
