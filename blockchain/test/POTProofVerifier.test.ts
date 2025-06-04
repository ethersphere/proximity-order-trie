import { expect } from "chai";
import { ethers } from "hardhat";
import { BaseContract, ContractFactory } from "ethers";

// Add this before your describe block
interface POTProofVerifierTesterContract extends BaseContract {
  isBitSetPublic(bitVector: string, bitIndex: number): Promise<boolean>;
  countOnesInBitVectorUntilPublic(bitVector: string, index: number): Promise<number>;
  calculatePOPublic(key1: string, key2: string, startPosition: number): Promise<number>;
  assertForkPathProof(proof: any): Promise<void>;
}

interface Proof {
  proofSegments: string[];
  proveSegment: string;
  chunkSpan: number;
}

interface EntryProof {
  bitVectorProof: Proof;
  entryProof: Proof;
}

interface ForkRefProof {
  bitVectorProof: Proof;
  forkReferenceProof: Proof;
}

interface ForkPathProof {
  rootReference: string;
  targetKey: string;
  forkRefProofs: ForkRefProof[];
  entryProof: EntryProof;
}

describe("POTProofVerifier", function () {
  let potProofVerifierTester: POTProofVerifierTesterContract;

  before(async function () {
    // Deploy the BMTChunk library first
    const BMTChunk: ContractFactory = await ethers.getContractFactory("BMTChunk");
    const bmtChunk = await BMTChunk.deploy();
    await bmtChunk.waitForDeployment();

    // Deploy the POTProofVerifier library
    const POTProofVerifier: ContractFactory = await ethers.getContractFactory("POTProofVerifier");
    const potProofVerifier = await POTProofVerifier.deploy();
    await potProofVerifier.waitForDeployment();

    // Deploy the test contract that uses the POTProofVerifier library
    const POTProofVerifierTester: ContractFactory = await ethers.getContractFactory("POTProofVerifierTester");

    potProofVerifierTester = await POTProofVerifierTester.deploy() as POTProofVerifierTesterContract;
    await potProofVerifierTester.waitForDeployment();
  });

  describe("Utility Functions", function () {
    it("should correctly check if a bit is set", async function () {
      // 1100 0000 1111 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000
      const bitVector = "0xC0F0000000000000000000000000000000000000000000000000000000000000";
      
      // check all true bits
      expect(await potProofVerifierTester.isBitSetPublic(bitVector, 0)).to.equal(true);
      expect(await potProofVerifierTester.isBitSetPublic(bitVector, 1)).to.equal(true);
      expect(await potProofVerifierTester.isBitSetPublic(bitVector, 8)).to.equal(true);
      
      // check some false bits
      expect(await potProofVerifierTester.isBitSetPublic(bitVector, 4)).to.equal(false);
      expect(await potProofVerifierTester.isBitSetPublic(bitVector, 6)).to.equal(false);
      expect(await potProofVerifierTester.isBitSetPublic(bitVector, 255)).      // Other bits should not be set
      to.equal(false);
    });

    it("should correctly count ones in a bit vector", async function () {
      // 0010 0101 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000
      const bitVector = "0x2500000000000000000000000000000000000000000000000000000000000000";
      
      expect(await potProofVerifierTester.countOnesInBitVectorUntilPublic(bitVector, 0)).to.equal(0);
      expect(await potProofVerifierTester.countOnesInBitVectorUntilPublic(bitVector, 2)).to.equal(0);
      expect(await potProofVerifierTester.countOnesInBitVectorUntilPublic(bitVector, 3)).to.equal(1);
      expect(await potProofVerifierTester.countOnesInBitVectorUntilPublic(bitVector, 5)).to.equal(1);
      expect(await potProofVerifierTester.countOnesInBitVectorUntilPublic(bitVector, 6)).to.equal(2);
      expect(await potProofVerifierTester.countOnesInBitVectorUntilPublic(bitVector, 7)).to.equal(2);
      expect(await potProofVerifierTester.countOnesInBitVectorUntilPublic(bitVector, 8)).to.equal(3);
      expect(await potProofVerifierTester.countOnesInBitVectorUntilPublic(bitVector, 256)).to.equal(3);
    });

    it("should correctly calculate proximity order", async function () {
      // Keys differ at bit 254
      const key1 = "0x0000000000000000000000000000000000000000000000000000000000000001"; // Differs at bit 255
      const key2 = "0x0000000000000000000000000000000000000000000000000000000000000003"; // Differs at bit 254
      
      // Keys differ at the last bit (bit 255)
      const key3 = "0x0000000000000000000000000000000000000000000000000000000000000001";
      const key4 = "0x0000000000000000000000000000000000000000000000000000000000000000";
      
      // Keys differ at first bit (bit 0)
      const key5 = "0x8000000000000000000000000000000000000000000000000000000000000000"; 
      const key6 = "0x0000000000000000000000000000000000000000000000000000000000000000";

      // PO should be the index of the first bit that differs
      expect(await potProofVerifierTester.calculatePOPublic(key1, key2, 0)).to.equal(254);
      expect(await potProofVerifierTester.calculatePOPublic(key3, key4, 0)).to.equal(255);
      expect(await potProofVerifierTester.calculatePOPublic(key5, key6, 0)).to.equal(0);
      
      // Test with start position
      expect(await potProofVerifierTester.calculatePOPublic(key1, key2, 254)).to.equal(254);
      expect(await potProofVerifierTester.calculatePOPublic(key1, key2, 255)).to.equal(256); // No difference after bit 255
    });
  });

  describe("Proof Verification", function () {
    // this proof is generated from the pkg/proof/forkpath_test.go 2 level entry case.
    const proof = require("./forkPathProofSample.json");

    it("should accept multi level entry proof", async function () {
      expect(await potProofVerifierTester.assertForkPathProof(proof)).not.to.be.reverted;
    })

    it("should revert for entry key does not match target key", async function () {
      const wrongProof = JSON.parse(JSON.stringify(proof));
      wrongProof.targetKey = "0x0020000000000000000000000000000000000000000000000000000000000000"
      await expect(potProofVerifierTester.assertForkPathProof(wrongProof)).to.be.revertedWith("Entry key does not match target key");
    })

    it("should revert for invalid bit vector proof at assertForkRefProof", async function () {
      const wrongProof = JSON.parse(JSON.stringify(proof));
      wrongProof.forkRefProofs[0].bitVectorProof.proveSegment = "0x0000000000000000000000000000000000000000000000000000000000000000"
      await expect(potProofVerifierTester.assertForkPathProof(wrongProof)).to.be.revertedWith("Fork is not set in the parent's bitvector");
    })

    it("should revert for invalid fork reference proof", async function () {
      const wrongProof = JSON.parse(JSON.stringify(proof));
      wrongProof.forkRefProofs[0].forkReferenceProof.proveSegment = "0x0000000000000000000000000000000000000000000000000000000000000000"
      await expect(potProofVerifierTester.assertForkPathProof(wrongProof)).to.be.revertedWith("Invalid fork reference proof");
    })

    it("should revert for invalid bit vector proof at assertForkRefProof", async function () {
      const wrongProof = JSON.parse(JSON.stringify(proof));
      wrongProof.forkRefProofs[0].bitVectorProof.proveSegment = "0xC000000000000000000000000000000000000000000000000000000000000000"
      await expect(potProofVerifierTester.assertForkPathProof(wrongProof)).to.be.revertedWith("Invalid bit vector proof at assertForkRefProof");
    })

    it("should revert for invalid entry proof", async function () {
      const wrongProof = JSON.parse(JSON.stringify(proof));
      wrongProof.entryProof.entryProof.proveSegment = "0x0000000000000000000000000000000000000000000000000000000000000000"
      await expect(potProofVerifierTester.assertForkPathProof(wrongProof)).to.be.revertedWith("Invalid entry proof");
    })

    it("should revert for invalid bit vector proof at assertEntryProof", async function () {
      const wrongProof = JSON.parse(JSON.stringify(proof));
      // Corrupt the proveSegment of the entryProof's bitVectorProof
      // This should cause the bitVectorHash in assertEntryProof to mismatch the currentNodeHash
      wrongProof.entryProof.bitVectorProof.proveSegment = "0x1111111111111111111111111111111111111111111111111111111111111111"; // A clearly different hash
      await expect(potProofVerifierTester.assertForkPathProof(wrongProof)).to.be.revertedWith("Invalid bit vector proof at assertEntryProof");
    });

  });
});
